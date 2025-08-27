package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"github.com/nubank/lola-ia-backend/internal"
	"github.com/nubank/lola-ia-backend/internal/provider"
	"github.com/nubank/lola-ia-backend/internal/store"
)

// buildFilesContext returns a compact context string about currently uploaded CSVs.
// It avoids sending large payloads by truncating content.
func buildFilesContext(mem *store.MemoryStore) string {
	files := mem.ListFiles()
	if len(files) == 0 {
		return ""
	}
	const (
		maxPerFileBytes = 20 * 1024 // include up to 20KB of each file
		maxTotalBytes   = 80 * 1024 // and cap overall context to ~80KB
	)
	var b strings.Builder
	b.WriteString("[Contexto de archivos CSV cargados]\n")
	b.WriteString("Puedes usar estos datos para responder si el usuario los menciona o pide análisis.\n")
	total := 0
	for _, f := range files {
		// encabezado por archivo
		fmt.Fprintf(&b, "- %s (%d bytes)\n", f.Name, f.Size)
		if total >= maxTotalBytes {
			continue
		}
		// contenido (truncado)
		txt := f.Text
		// limitar per-file
		if len(txt) > maxPerFileBytes {
			txt = txt[:maxPerFileBytes]
		}
		// asegurar límites totales
		if total+len(txt) > maxTotalBytes {
			txt = txt[:maxTotalBytes-total]
		}
		// evitar cortar runas UTF-8 por la mitad
		for !utf8.ValidString(txt) && len(txt) > 0 {
			txt = txt[:len(txt)-1]
		}
		if len(txt) > 0 {
			b.WriteString("Contenido (parcial):\n\n")
			b.WriteString(txt)
			b.WriteString("\n\n")
			total += len(txt)
		}
	}
	return b.String()
}

// preloadSeedCSVs scans a directory for .csv files and loads them into memory.
// It returns the number of files added. Non-fatal errors are logged to stdout.
func preloadSeedCSVs(dir string, mem *store.MemoryStore) int {
	if dir == "" {
		return 0
	}
	// Check dir exists
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		fmt.Printf("[seed] carpeta no válida: %s\n", dir)
		return 0
	}
	// Gather CSV files
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Printf("[seed] error leyendo dir: %v\n", err)
		return 0
	}

	files := make([]internal.KnowledgeFile, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(strings.ToLower(name), ".csv") {
			p := filepath.Join(dir, name)
			b, err := os.ReadFile(p)
			if err != nil {
				fmt.Printf("[seed] error leyendo %s: %v\n", name, err)
				continue
			}

			files = append(files, internal.KnowledgeFile{Name: name, Size: len(b), Text: string(b)})
		}
	}
	if len(files) == 0 {
		return 0
	}
	// Respect simple max limit used by POST /api/files
	if len(mem.ListFiles())+len(files) > filesMax {
		// trim to available slots
		slots := filesMax - len(mem.ListFiles())
		if slots <= 0 {
			return 0
		}
		files = files[:slots]
	}
	total := mem.AddFiles(files)
	fmt.Printf("[seed] precargados %d CSV(s) desde %s (total en memoria: %d)\n", len(files), dir, total)
	return len(files)
}

// Analyst prompt template (raw string). Fill placeholders with user query and CSV context.
const analystTemplate = `You are an expert market researcher and data analyst for a major financial institution. Your task is to analyze raw customer feedback and summarize the key insights. Below is a collection of customer feedback data from various sources including social media, surveys, and chat logs.
Customer Data: {Insert your raw customer data here}
User Query: {Insert the Nubanker's question here, e.g., "what are credit card customers' main pain points from the last 3 months?"}
Instructions:
Analyze the provided "Customer Data" to answer the "User Query."
Synthesize the key information into a concise summary.
Identify the main pain points, frustrations, and underlying customer needs mentioned in the data.
Translate the pain points into specific, actionable feedback that can be used by product and operations teams
List the top 3 most frequently mentioned topics or themes related to the query. For each topic, calculate the approximate percentage of mentions it accounts for.
For each of the top 3 topics, provide 1-2 direct quotes (verbatim) from the data to serve as concrete examples.

Mode rules:
- Use the required output format ONLY if the User Query is about analyzing data/feedback (e.g., asks for insights, summary, pain points, frequencies/percentages, themes/topics, verbatim quotes, surveys, social listening, or similar analysis tasks).
- If the User Query is NOT about data analysis (e.g., greetings, casual questions, UI/help questions, deployment, configuration), DO NOT use the formatted sections. Respond briefly and directly in neutral Spanish without any of the formatted headers.
- If there is no relevant Customer Data for the User Query, say so concisely and still follow the previous rule about whether to use the formatted sections.

Output language: Respond strictly in neutral Spanish.

When the analysis mode applies, format the final response using the exact structure below. Do not include any extra text, introductions, or conclusions outside of this format.
Format:
--- Summary [Provide a concise, high-level summary here.]
--- Main Pain Points & Needs [List the main pain points and needs using bullet points.]
--- Actionable Feedback [List actionable feedback using bullet points.]
--- Top 3 Topics and (%) of Mentions [List the topics with their percentage here, e.g., 1. Topic One (X%) 2. Topic Two (Y%) 3. Topic Three (Z%) ]
--- Examples of Verbatim for those main topics [Provide verbatim examples here, clearly separating them by topic.]`

func buildAnalystPrompt(userQuery, csvContext string) string {
	// Insert CSV context and user query into the template
	s := strings.Replace(analystTemplate, "{Insert your raw customer data here}", csvContext, 1)
	s = strings.Replace(s, "{Insert the Nubanker's question here, e.g., \"what are credit card customers' main pain points from the last 3 months?\"}", userQuery, 1)
	return s
}

// Heuristic: detect if the user query asks for analysis/insights rather than casual chat.
func isAnalystQuery(q string) bool {
	ql := strings.ToLower(q)
	keywords := []string{
		"analiza", "análisis", "analysis", "analizar", "insights", "resumen", "summary",
		"puntos de dolor", "pain points", "temas", "topics", "top 3", "top3", "%", "porcentaje",
		"frecuencia", "tendencias", "trends", "verbatim", "citas", "quotes", "encuesta", "surveys",
		"feedback", "quejas", "needs", "necesidades", "social", "menciones", "cluster", "tema",
		"csv", "datos", "data"}
	for _, kw := range keywords {
		if strings.Contains(ql, kw) {
			return true
		}
	}
	return false
}

const filesMax = 50

func main() {
	_ = godotenv.Load() // carga .env si existe

	r := gin.Default()

	// CORS con credenciales: permite localhost, el front en ngrok y *.vercel.{app,dev}
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, ngrok-skip-browser-warning")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "*")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// Store en memoria (MVP sin auth)
	mem := store.NewMemoryStore()
	store.SeedAssistantHello(mem, "¡Hola! Soy Lola IA lista para ayudarte 🚀")

	// Precarga de CSVs desde carpeta (opcional)
	seedDir := os.Getenv("SEED_CSV_DIR")
	if seedDir == "" {
		seedDir = "./seed"
	}
	_ = preloadSeedCSVs(seedDir, mem)

	// Feature flag to enable analyst formatting mode
	useAnalyst := true

	// Provider (OpenAI si hay API key, si no: mock)
	var chat provider.ChatProvider
	if _, ok := os.LookupEnv("OPENAI_API_KEY"); ok {
		mdl := os.Getenv("OPENAI_MODEL")
		p, err := provider.NewOpenAIProvider(mdl)
		if err == nil {
			chat = p
		}
	}
	if chat == nil {
		chat = provider.MockProvider{}
	}

	// Rutas
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true, "uptime": time.Now().Format(time.RFC3339)})
	})

	r.GET("/api/model", func(c *gin.Context) {
		c.JSON(200, gin.H{"model": chat.Model()})
	})

	r.GET("/api/messages", func(c *gin.Context) {
		c.JSON(200, internal.ChatHistory{Messages: mem.All()})
	})

	r.POST("/api/messages", func(c *gin.Context) {
		var req internal.SendMessageRequest
		if err := c.BindJSON(&req); err != nil || req.Content == "" {
			c.JSON(400, gin.H{"error": "content requerido"})
			return
		}

		// Guardamos mensaje del usuario
		userMsg := internal.Message{
			Role:      internal.RoleUser,
			Content:   req.Content,
			CreatedAt: time.Now(),
		}
		mem.Append(userMsg)

		// Construimos el prompt final conmutando modo análisis si aplica
		var prompt string
		if useAnalyst && isAnalystQuery(req.Content) {
			csvCtx := buildFilesContext(mem)
			prompt = buildAnalystPrompt(req.Content, csvCtx)
		} else {
			// Modo normal: no forzamos formato ni añadimos CSV para preguntas casuales
			prompt = req.Content
		}
		replyText, err := chat.Reply(mem.All(), prompt)
		if err != nil {
			c.JSON(502, gin.H{"error": err.Error()})
			return
		}

		assistantMsg := internal.Message{
			Role:      internal.RoleAssistant,
			Content:   replyText,
			CreatedAt: time.Now(),
		}
		mem.Append(assistantMsg)

		c.JSON(200, internal.SendMessageResponse{
			Reply: assistantMsg,
			Model: chat.Model(),
		})
	})

	r.POST("/api/reset", func(c *gin.Context) {
		mem.Reset()
		store.SeedAssistantHello(mem, "He reiniciado la conversación. ¿En qué te ayudo?")
		c.JSON(200, gin.H{"ok": true})
	})

	// Archivos CSV (knowledge base)
	r.GET("/api/files", func(c *gin.Context) {
		c.JSON(200, gin.H{"files": mem.ListFiles()})
	})

	r.POST("/api/files", func(c *gin.Context) {
		var req internal.UploadFilesRequest
		if err := c.BindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "JSON inválido o vacío"})
			return
		}
		if len(req.Files) == 0 {
			c.JSON(400, gin.H{"error": "files requerido"})
			return
		}
		// límite simple para MVP
		current := len(mem.ListFiles())
		incoming := len(req.Files)
		if current+incoming > filesMax {
			c.JSON(413, gin.H{"error": "se excede el máximo de archivos", "max": filesMax})
			return
		}
		total := mem.AddFiles(req.Files)
		c.JSON(200, internal.UploadFilesResponse{Count: len(req.Files), Total: total})
	})

	r.DELETE("/api/files", func(c *gin.Context) {
		mem.ClearFiles()
		c.JSON(200, gin.H{"ok": true})
	})

	r.DELETE("/api/files/:name", func(c *gin.Context) {
		name := c.Param("name")
		left := mem.RemoveFile(name)
		c.JSON(200, gin.H{"total": left})
	})

	// Puerto
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	_ = r.Run(":" + port)
}
