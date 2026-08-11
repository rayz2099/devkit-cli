package cli

const CurrentVersion = "v0.2.1"

func IsVersionRequest(args []string) bool {
	if len(args) != 1 {
		return false
	}

	switch args[0] {
	case "--version", "version":
		return true
	default:
		return false
	}
}

func VersionText() string {
	return CurrentVersion + "\n"
}
