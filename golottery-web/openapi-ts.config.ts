import { defineConfig } from "@hey-api/openapi-ts";

export default defineConfig({
  input: "../golottery-api/api/openapi.yaml",
  output: {
    path: "src/api-gen",
    clean: true,
  },
  plugins: ["@hey-api/client-fetch"],
});
