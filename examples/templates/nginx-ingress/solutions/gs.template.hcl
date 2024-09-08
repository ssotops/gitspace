template "solutions" {
  version = "1.0.0"
  description = "Solution-specific configuration for NGINX Ingress Controller"
  author = "Your Organization"

  parent = "nginx-ingress"

  files = [
    "values.yaml",
    "README.md"
  ]

  tokens = [
    {
      name = "SOLUTION_NAME"
      files = ["values.yaml", "README.md"]
      encoding = "plaintext"
      phase = "pre-deploy"
    },
    {
      name = "INGRESS_CLASS"
      files = ["values.yaml"]
      encoding = "plaintext"
      phase = "deploy"
    },
    {
      name = "TLS_SECRET"
      files = ["values.yaml"]
      encoding = "base64"
      phase = "post-deploy"
    }
  ]
}
