gitspace {
  path = "gs"
  labels = ["feature", "bug"]
  clone {
    scm = "github.com"
    owner = "ssotops"
    auth {
      type = "ssh"
      keyPath = "$SSH_KEY_PATH"
    }

    startsWith {
      group "git" {
        values = ["git"]
          type = "gitops"
          labels = ["backend", "core"]
      }
    }

    endsWith {
      group "space" {
        values = ["space"]
        type = "solution"
        labels = ["frontend", "experimental"]
      } 
    }

    includes {
      group "sso" {
        values = ["sso"]
        type = "ssot"
        labels = ["auth", "security"]
      }
      group "plugins" {
        values = ["-plugin-"]
      }
      group "templates" {
        values = ["-template-"]
      }
    }

    isExactly { // Changed from name to isExactly
      group "scmany" {
        values = ["scmany"]
        type = "helper"
        labels = ["utility"]
      }
    }
  }
}
