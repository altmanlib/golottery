import { defineConfig } from "@hey-api/openapi-ts";

export default defineConfig({
  input: "http://127.0.0.1:5568/openapi.json",
  output: {
    path: "src/api-gen",
    clean: true,
  },
  plugins: ["@hey-api/client-fetch"],
});
