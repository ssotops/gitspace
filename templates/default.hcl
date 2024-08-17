repositories {
  gitspace {
    path = "gs"
  }
  clone {
    scm = "github.com"
    owner = "ssotops"
    startsWith = ["git"]
    endsWith = ["space"]
    includes = ["sso"]
    name = ["gitspace", "ssotspace"]
    auth {
      type = "ssh"
      keyPath = "~/.ssh/alechp"
    }
  }
}
