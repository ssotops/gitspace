module github.com/ssotops/gitspace

go 1.23.1

require (
	code.gitea.io/sdk/gitea v0.19.0
	github.com/charmbracelet/bubbles v0.20.0
	github.com/charmbracelet/bubbletea v1.1.1
	github.com/charmbracelet/huh v0.6.0
	github.com/charmbracelet/lipgloss v0.13.0
	github.com/charmbracelet/log v0.4.0
	github.com/go-git/go-git/v5 v5.12.0
	github.com/google/go-github/v39 v39.2.0
	github.com/mitchellh/go-homedir v1.1.0
	github.com/pelletier/go-toml v1.9.5
	github.com/pelletier/go-toml/v2 v2.2.3
	github.com/ssotops/gitspace-plugin-sdk v0.0.0-20241001023129-8c91f9f5d979
	golang.org/x/crypto v0.27.0
	golang.org/x/oauth2 v0.23.0
	google.golang.org/protobuf v1.34.2
)

require (
	dario.cat/mergo v1.0.1 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/ProtonMail/go-crypto v1.0.0 // indirect
	github.com/atotto/clipboard v0.1.4 // indirect
	github.com/aymanbagabas/go-osc52/v2 v2.0.1 // indirect
	github.com/catppuccin/go v0.2.0 // indirect
	github.com/charmbracelet/x/ansi v0.3.2 // indirect
	github.com/charmbracelet/x/exp/strings v0.0.0-20240919170804-a4978c8e603a // indirect
	github.com/charmbracelet/x/term v0.2.0 // indirect
	github.com/cloudflare/circl v1.4.0 // indirect
	github.com/cyphar/filepath-securejoin v0.3.3 // indirect
	github.com/davidmz/go-pageant v1.0.2 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/erikgeiser/coninput v0.0.0-20211004153227-1c3628e74d0f // indirect
	github.com/go-fed/httpsig v1.1.0 // indirect
	github.com/go-git/gcfg v1.5.1-0.20230307220236-3a3c6141e376 // indirect
	github.com/go-git/go-billy/v5 v5.5.0 // indirect
	github.com/go-logfmt/logfmt v0.6.0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/kevinburke/ssh_config v1.2.0 // indirect
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-localereader v0.0.1 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/mitchellh/hashstructure/v2 v2.0.2 // indirect
	github.com/muesli/ansi v0.0.0-20230316100256-276c6243b2f6 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/muesli/termenv v0.15.3-0.20240618155329-98d742f6907a // indirect
	github.com/pjbgf/sha1cd v0.3.0 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/rogpeppe/go-internal v1.12.0 // indirect
	github.com/sergi/go-diff v1.3.2-0.20230802210424-5b0b94c5c0d3 // indirect
	github.com/skeema/knownhosts v1.3.0 // indirect
	github.com/xanzy/ssh-agent v0.3.3 // indirect
	golang.org/x/exp v0.0.0-20240909161429-701f63a606c0 // indirect
	golang.org/x/net v0.29.0 // indirect
	golang.org/x/sync v0.8.0 // indirect
	golang.org/x/sys v0.25.0 // indirect
	golang.org/x/text v0.18.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240930140551-af27646dc61f // indirect
	google.golang.org/grpc v1.67.1 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
)

exclude github.com/ssotops/gitspace/examples/plugins v0.0.0

replace github.com/ssotops/gitspace/lib => ./lib

replace github.com/ssotops/gitspace-plugin-sdk => ./gs/gitspace-plugin-sdk
