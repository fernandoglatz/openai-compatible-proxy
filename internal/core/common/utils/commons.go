package utils

import (
	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils/constants"
	"os"
	"strings"
)

func IsEmptyStr(value string) bool {
	return len(value) == constants.ZERO
}

func IsNotEmptyStr(value string) bool {
	return len(value) > constants.ZERO
}

func IsBlankStr(value string) bool {
	return IsEmptyStr(strings.TrimSpace(value))
}

func IsNotBlankStr(value string) bool {
	return IsNotEmptyStr(strings.TrimSpace(value))
}

func GetTimezone() string {
	return os.Getenv("TZ")
}

// ExtractRootDomain extracts the main domain from a hostname, handling common subdomains and public suffixes.
// For example, www.strava.com -> strava.com, api.example.co.uk -> example.co.uk
func ExtractRootDomain(hostname string) string {
	parts := strings.Split(hostname, ".")
	if len(parts) < 2 {
		return hostname
	}

	// List of common public suffixes with two parts (expand as needed)
	publicSuffixes := map[string]bool{
		"co.uk":  true,
		"com.br": true,
		"com.au": true,
		"org.uk": true,
		"gov.uk": true,
		"ac.uk":  true,
	}

	lastTwo := strings.Join(parts[len(parts)-2:], ".")
	lastThree := ""
	if len(parts) >= 3 {
		lastThree = strings.Join(parts[len(parts)-3:], ".")
	}

	if publicSuffixes[lastTwo] && len(parts) >= 3 {
		return strings.Join(parts[len(parts)-3:], ".")
	}
	if publicSuffixes[lastThree] && len(parts) >= 4 {
		return strings.Join(parts[len(parts)-4:], ".")
	}
	return lastTwo
}
