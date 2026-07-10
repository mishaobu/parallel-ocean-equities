import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  base: "/monetary/",
  plugins: [react()],
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes("node_modules")) return undefined;
          if (id.includes("/recharts/")) return "recharts";
          if (id.includes("/d3-") || id.includes("/victory-vendor/")) return "chart-vendor";
          if (id.includes("/lucide-react/")) return "icons";
          return "vendor";
        },
      },
    },
  },
  server: {
    host: "127.0.0.1",
    port: 5174,
    proxy: { "/equities/api": "http://127.0.0.1:8080" },
  },
});
