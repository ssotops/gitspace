variable "repository_types" {
  type = map(string)
  default = {
    gitops   = "gitops"
    solution = "solution"
    helper   = "helper"
    ssot     = "ssot"
  }
}

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
        type = var.repository_types.gitops
        labels = ["backend", "core"]
      }
    }

    endsWith {
      values = ["space"]
      repository {
        type = var.repository_types.solution
        labels = ["frontend", "experimental"]
      }
    }

    includes {
      values = ["sso"]
      repository {
        type = var.repository_types.ssot
        labels = ["auth", "security"]
      }
    }

    name {
      values = ["scmany"]
      repository {
        type = var.repository_types.helper
        labels = ["utility"]
      }
    }

    auth {
      type = "ssh"
      keyPath = "$SSH_KEY_PATH"
    }
  }
}
