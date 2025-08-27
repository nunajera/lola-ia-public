import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    allowedHosts: [
      '*.ngrok-free.app', '98c465640a25.ngrok-free.app', 'lola-ia.ngrok.app', 'lola-api.ngrok.app'
    ]
  }
});