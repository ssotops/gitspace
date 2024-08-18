repositories {
  gitspace {
    path = "gs"
  }
  clone {
    scm = "github.com"
    owner = "ssotops"
    # repositories starting with "git"
    # eg. github.com/ssotops/gitspace
    startsWith = ["git"]

    # repositories ending with "space", 
    # eg. github.com/ssotops/gitspace, github.com/ssotops/k1space, github.com/ssotops/ssotspace
    endsWith = ["space"]

    # repositories containing 
    # eg. github.com/ssotops/ssotspace
    includes = ["sso"]

    # repositories named "scmany"
    # eg. github.com/ssotops/scmany
    name = ["scmany"]
    auth {
      type = "ssh"
      keyPath = "~/.ssh/my-key"
    }
  }
}
