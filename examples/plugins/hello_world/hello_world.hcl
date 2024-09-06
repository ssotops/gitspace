plugin "hello_world" {
  version = "1.0.0"
  description = "A simple Hello World plugin for Gitspace"
  author = "Gitspace Team"
  
  entry_point = "HelloWorld"
  
  source {
    type = "github"
    repository = "ssotops/gitspace-hello-world-plugin"
    branch = "main"
  }
}
