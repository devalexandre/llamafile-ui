import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import wails from "@wailsio/runtime/plugins/vite";

export default defineConfig({
  server: {
    host: "127.0.0.1",
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
    // Translations live in internal/i18n/locales (shared with the Go backend).
    fs: { allow: [".."] },
  },
  plugins: [react(), tailwindcss(), wails("./bindings")],
});
