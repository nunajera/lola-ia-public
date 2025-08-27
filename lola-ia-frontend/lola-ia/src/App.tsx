import { useEffect, useRef, useState } from "react";

type Role = "user" | "assistant";
interface Message {
  role: Role;
  content: string;
  created_at?: string;
}

interface CsvAttachment { name: string; size: number; text: string }
interface FilesListResp { files: CsvAttachment[] }
interface UploadFilesResp { count: number; total: number }

function formatBytes(b: number) {
  if (b < 1024) return `${b} B`;
  const kb = b / 1024;
  if (kb < 1024) return `${kb.toFixed(1)} KB`;
  const mb = kb / 1024;
  return `${mb.toFixed(2)} MB`;
}

// Puedes sobreescribir con Vite: VITE_API_BASE="http://localhost:8080"
const API_BASE = (import.meta as any).env?.VITE_API_BASE || "http://localhost:8080";


async function api<T>(path: string, opts?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    headers: {
      "Content-Type": "application/json",
      "ngrok-skip-browser-warning": "true",
    },
    credentials: "include",
    ...opts,
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `HTTP ${res.status}`);
  }
  return res.json();
}

function App() {
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState("");
  const [model, setModel] = useState("Lola IA");
  const [sending, setSending] = useState(false);
  const endRef = useRef<HTMLDivElement | null>(null);
  const [attachments, setAttachments] = useState<CsvAttachment[]>([]);
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const textareaRef = useRef<HTMLTextAreaElement | null>(null);
  const MIN_TA_HEIGHT = 40; // px (~2.5rem)
  const MAX_TA_HEIGHT = 160; // px (~10rem)
  const autosizeTextarea = () => {
    const el = textareaRef.current;
    if (!el) return;
    el.style.height = "auto";
    const next = Math.min(el.scrollHeight, MAX_TA_HEIGHT);
    el.style.height = `${Math.max(next, MIN_TA_HEIGHT)}px`;
    el.style.overflowY = el.scrollHeight > MAX_TA_HEIGHT ? "auto" : "hidden";
  };

  // Cargar modelo + historial al montar
  useEffect(() => {
    (async () => {
      try {
        const m = await api<{ model: string }>("/api/model");
        if (m?.model) setModel(m.model);
      } catch {}
      try {
        const h = await api<{ messages: Message[] }>("/api/messages");
        if (Array.isArray(h?.messages)) setMessages(h.messages);
      } catch (e) {
        console.error(e);
      }
      try {
        const fl = await api<FilesListResp>("/api/files");
        if (Array.isArray(fl?.files)) setAttachments(fl.files);
      } catch (e) {
        console.error(e);
      }
    })();
  }, []);

  // Autoscroll al último mensaje
  useEffect(() => {
    endRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  useEffect(() => {
    autosizeTextarea();
  }, [input]);

  const handleAttachClick = () => {
    fileInputRef.current?.click();
  };
  
  const handleFilesSelected = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(e.target.files || []).filter(f =>
      f.name.toLowerCase().endsWith(".csv")
    );
    if (!files.length) return;

    const reads = files.map(f => new Promise<CsvAttachment>((resolve, reject) => {
      const fr = new FileReader();
      fr.onload = () => resolve({ name: f.name, size: f.size, text: String(fr.result || "") });
      fr.onerror = () => reject(fr.error);
      fr.readAsText(f);
    }));

    try {
      const loaded = await Promise.all(reads);
      // Optimistic UI
      setAttachments(prev => {
        const map = new Map(prev.map(a => [a.name, a]));
        for (const a of loaded) map.set(a.name, a);
        return Array.from(map.values());
      });
      // Backend sync
      await api<UploadFilesResp>("/api/files", {
        method: "POST",
        body: JSON.stringify({ files: loaded })
      });
    } catch (err) {
      console.error("Error leyendo/subiendo CSV:", err);
    } finally {
      if (fileInputRef.current) fileInputRef.current.value = "";
    }
  };
  
  const removeAttachment = async (name: string) => {
    // Optimistic update
    setAttachments(prev => prev.filter(a => a.name !== name));
    try {
      await api<{ total: number }>(`/api/files/${encodeURIComponent(name)}`, { method: "DELETE" });
    } catch (e) {
      console.error(e);
    }
  };

  const clearAttachments = async () => {
    setAttachments([]);
    try {
      await api<{ ok: boolean }>("/api/files", { method: "DELETE" });
    } catch (e) {
      console.error(e);
    }
  };

  const handleSend = async () => {
    const content = input.trim();
    if (!content || sending) return;

    setInput("");
    setSending(true);

    // Optimistic UI: agregamos el mensaje del usuario al instante
    setMessages((prev) => [...prev, { role: "user", content }]);

    try {
      const resp = await api<{ reply: Message; model: string }>("/api/messages", {
        method: "POST",
        body: JSON.stringify({ content }),
      });
      if (resp?.model) setModel(resp.model);
      if (resp?.reply) {
        setMessages((prev) => [...prev, resp.reply]);
      }
    } catch (err) {
      console.error(err);
      setMessages((prev) => [
        ...prev,
        { role: "assistant", content: "(Error al obtener respuesta del servidor)" },
      ]);
    } finally {
      setSending(false);
    }
  };

  const handleReset = async () => {
    try {
      await api<{ ok: boolean }>("/api/reset", { method: "POST" });
      const h = await api<{ messages: Message[] }>("/api/messages");
      setMessages(h.messages || []);
    } catch (e) {
      console.error(e);
    }
  };

  return (
    <div className="min-h-screen bg-[#0B0B0C] text-gray-200">
      {/* Top bar minimal */}
      <div className="sticky top-0 z-10 backdrop-blur supports-[backdrop-filter]:bg-black/60 bg-black/40 border-b border-white/10">
        <div className="mx-auto max-w-6xl px-4">
          <div className="h-14 flex items-center">
            <div className="flex items-center gap-3 relative">
              <div className="h-8 w-8 rounded-full bg-gradient-to-br from-purple-500 to-fuchsia-500 grid place-items-center font-bold">l</div>
              <span className="text-sm text-gray-400">Lola IA · studio</span>
              {/* Purple dot */}
              <span className="absolute -bottom-1 left-5 h-3 w-3 rounded-full bg-purple-400 border border-black"></span>
            </div>
          </div>
        </div>
      </div>

      {/* Window / Card container */}
      <main className="flex justify-center px-4 sm:px-6 py-8">
        <div className="w-full max-w-4xl lg:max-w-5xl rounded-2xl border border-white/10 bg-white/5 backdrop-blur-xl shadow-[0_0_0_1px_rgba(255,255,255,0.06)]">
          {/* Window header with model + reset */}
          <div className="flex items-center justify-between p-5 md:p-6">
            <div className="flex items-center gap-2 text-xs text-gray-400">
              <span className="h-2 w-2 rounded-full bg-purple-400"></span>
              <span>Modelo: {model}</span>
            </div>
            <div className="flex items-center gap-2">
              <button
                onClick={handleReset}
                className="px-3 py-1.5 text-xs rounded-lg bg-white/5 border border-white/10 hover:bg-white/10"
              >
                Reset
              </button>
            </div>
          </div>

          {/* Divider */}
          <div className="h-px bg-white/10" />

          {/* Chat area */}
          <div className="p-4 space-y-4">
            {messages.map((msg, i) => (
              <div key={i} className={`flex items-start gap-3 ${msg.role === "assistant" ? "justify-start" : "justify-end"}`}>
                {msg.role === "assistant" && (
                  <div className="h-8 w-8 flex-shrink-0 rounded-full bg-gradient-to-br from-purple-600 to-fuchsia-600 grid place-items-center text-sm font-bold relative">
                    l
                    <span className="absolute -bottom-1 right-0 h-2.5 w-2.5 rounded-full bg-purple-400 border border-black"></span>
                  </div>
                )}

                <div
                  className={
                    msg.role === "assistant"
                      ? "max-w-[85%] md:max-w-[75%] rounded-2xl px-4 py-3 bg-white/5 border border-white/10 text-gray-100"
                      : "max-w-[85%] md:max-w-[75%] rounded-2xl px-4 py-3 bg-gradient-to-br from-purple-700 to-fuchsia-700 text-white shadow"
                  }
                >
                  <div className="whitespace-pre-wrap break-words">{msg.content}</div>
                </div>

                {msg.role === "user" && (
                  <div className="h-8 w-8 flex-shrink-0 rounded-full bg-white/10 border border-white/10 grid place-items-center text-sm text-gray-400">👤</div>
                )}
              </div>
            ))}
            <div ref={endRef} />
          </div>

          {/* Selected CSVs chips */}
          {attachments.length > 0 && (
            <div className="px-4 py-3 border-t border-white/10">
              <div className="flex items-center justify-between pb-2">
                <span className="text-xs text-gray-400">
                  Archivos CSV cargados ({attachments.length})
                </span>
                <button
                  onClick={clearAttachments}
                  className="text-xs text-gray-400 hover:text-gray-200"
                  type="button"
                >
                  Limpiar
                </button>
              </div>
              <div className="flex flex-wrap gap-2">
                {attachments.map(a => (
                  <div
                    key={a.name}
                    className="group flex items-center gap-2 text-xs rounded-full border border-white/10 bg-white/5 px-3 py-1"
                  >
                    <span className="truncate max-w-[16rem]" title={a.name}>{a.name}</span>
                    <span className="text-gray-500">· {formatBytes(a.size)}</span>
                    <button
                      onClick={() => removeAttachment(a.name)}
                      className="ml-1 rounded hover:bg-white/10 px-1"
                      title="Quitar"
                      type="button"
                    >
                      ✕
                    </button>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Composer integrated in window */}
          <div className="border-t border-white/10 p-4 md:p-5">
            <div className="flex items-center gap-2 rounded-xl border border-white/10 bg-white/5 px-2 py-2">
              <button
                onClick={handleAttachClick}
                className="p-2 rounded-lg hover:bg-white/10"
                aria-label="Adjuntar"
                type="button"
              >
                📎
              </button>
              <input
                ref={fileInputRef}
                type="file"
                accept=".csv"
                multiple
                className="hidden"
                onChange={handleFilesSelected}
              />
              <textarea
                ref={textareaRef}
                className="flex-1 bg-transparent outline-none text-gray-200 placeholder:text-gray-500 px-1 resize-none leading-relaxed"
                placeholder="Escribe tu mensaje…"
                value={input}
                onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) => setInput(e.target.value)}
                onInput={autosizeTextarea}
                onKeyDown={(e) => {
                  if (e.key === "Enter" && !e.shiftKey) {
                    e.preventDefault();
                    handleSend();
                    setTimeout(() => autosizeTextarea(), 0);
                  }
                }}
                disabled={sending}
                rows={1}
                aria-label="Escribe tu mensaje"
                style={{ height: `${MIN_TA_HEIGHT}px`, maxHeight: `${MAX_TA_HEIGHT}px`, overflowY: "hidden" }}
              />
              <button
                onClick={handleSend}
                disabled={sending || !input.trim()}
                className="px-3 py-2 rounded-lg bg-gradient-to-r from-purple-600 to-fuchsia-500 hover:from-purple-500 hover:to-fuchsia-400 text-white disabled:opacity-60 disabled:cursor-not-allowed"
              >
                {sending ? "…" : attachments.length ? "📤" : "➤"}
              </button>
            </div>
            <div className="mt-2 px-2 text-[11px] text-gray-500">Lola IA puede cometer errores. Verifica información importante.</div>
          </div>
        </div>
      </main>

      {/* Footer */}
      <footer className="mx-auto max-w-6xl px-4 pb-8 text-xs text-gray-500 flex items-center justify-between">
        <span>© 2025 Lola IA · studio</span>
        <div className="flex gap-6">
          <a href="#" className="hover:text-gray-300">Privacidad</a>
          <a href="#" className="hover:text-gray-300">Términos</a>
          <a href="#" className="hover:text-gray-300">Contacto</a>
        </div>
      </footer>
    </div>
  );
}

export default App;