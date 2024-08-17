repositories {
  gitspace {
    path = "gs"
  }
  clone {
    scm = "github.com"
    owner = "ssotops"
    # repositories starting with "git"
    # startsWith = ["git"]

    # repositories ending with "space"
    endsWith = ["space"]

    # repositories containing "sso"
    # includes = ["sso"]

    # repositories named "gitspace" or "ssotspace"
    # name = ["gitspace", "ssotspace"]
    auth {
      type = "ssh"
      keyPath = "~/.ssh/alechp"
    }
  }
}
