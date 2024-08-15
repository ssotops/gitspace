# Which repositories to clone
repositories {
  # Create symlinks
  gitspace {
    path = "gs"
  }
  clone {
    # Remote source
    scm = "github"
    # owner
    owner = "ssotspace"
    # List of suffixes to include
    endsWith = [
      "space"
    ]
  }
}
