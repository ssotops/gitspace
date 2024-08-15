repositories {
  gitspace {
    path = "gs"
  }
  clone {
    scm = "github.com"
    owner = "ssotspace"
    endsWith = ["space"]
    auth {
      type = "ssh"
      keyPath = "~/.ssh/alechp"
    }
  }
}
