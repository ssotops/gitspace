repositories {
  gitspace {
    path = "gs"
  }
  labels = ["feature", "bug"]
  clone {
    scm = "github.com"
    owner = "ssotops"
    
    startsWith {
      values = ["git"]
      repository {
        type = "gitops"
        labels = ["backend", "core"]
      }
    }

    endsWith {
      values = ["space"]
      repository {
        type = "solution"
        labels = ["frontend", "experimental"]
      }
    }

    includes {
      values = ["sso"]
      repository {
        type = "ssot"
        labels = ["auth", "security"]
      }
    }

    name {
      values = ["scmany"]
      repository {
        type = "helper"
        labels = ["utility"]
      }
    }

    auth {
      type = "ssh"
      keyPath = "$SSH_KEY_PATH"
    }
  }
}
