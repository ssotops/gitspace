repositories {
  gitspace {
    path = "gs"
  }
  clone {
    scm = "github.com"
    owner = "ssotops"
    endsWith = ["space"]
    auth {
      type = "ssh"
      # update this to your ssh key path
      keyPath = "~/.ssh/alechp"
    }
  }
}
