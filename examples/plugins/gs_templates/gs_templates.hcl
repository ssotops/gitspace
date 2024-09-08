// template_plugin.hcl
plugin "template" {
  version = "1.0.0"
  description = "A Template Plugin for Gitspace"
  author = "Gitspace Team"
  
  entry_point = "Template"
  
  source {
    type = "github"
    repository = "ssotops/gitspace-template-plugin"
    branch = "main"
  }
}
