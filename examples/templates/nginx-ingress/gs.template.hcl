template "nginx-ingress" {
  version = "1.0.0"
  description = "NGINX Ingress Controller template"
  author = "Your Organization"

  child_templates = ["gitops", "solutions"]

  lifecycle_phases = ["pre-deploy", "deploy", "post-deploy"]
}