template "gitops" {
  version = "1.0.0"
  description = "GitOps configuration for NGINX Ingress Controller"
  author = "Your Organization"

  parent = "nginx-ingress"

  files = [
    "deployment.yaml",
    "service.yaml",
    "ingress.yaml"
  ]

  tokens = [
    {
      name = "NGINX_VERSION"
      files = ["deployment.yaml"]
      encoding = "plaintext"
      phase = "deploy"
    },
    {
      name = "REPLICAS"
      files = ["deployment.yaml"]
      encoding = "plaintext"
      phase = "deploy"
    },
    {
      name = "SERVICE_NAME"
      files = ["service.yaml", "ingress.yaml"]
      encoding = "plaintext"
      phase = "pre-deploy"
    }
  ]
}