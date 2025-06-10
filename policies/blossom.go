package policies

import (
	"context"
	"strings"
)

// RedirectGet returns a function that redirects to a specified URL template with the given status code.
// The URL template can include {sha256} and/or {extension} placeholders that will be replaced
// with the actual values. If neither placeholder is present, {sha256}.{extension} will be
// appended to the URL with proper forward slash handling.
func RedirectGet(urlTemplate string, statusCode int) func(context.Context, string, string) (url string, code int, err error) {
	return func(ctx context.Context, sha256 string, extension string) (string, int, error) {
		finalURL := urlTemplate

		// Replace placeholders if they exist
		hasSHA256Placeholder := strings.Contains(finalURL, "{sha256}")
		hasExtensionPlaceholder := strings.Contains(finalURL, "{extension}")

		if hasSHA256Placeholder {
			finalURL = strings.Replace(finalURL, "{sha256}", sha256, -1)
		}

		if hasExtensionPlaceholder {
			finalURL = strings.Replace(finalURL, "{extension}", extension, -1)
		}

		// If neither placeholder is present, append sha256.extension
		if !hasSHA256Placeholder && !hasExtensionPlaceholder {
			// Ensure URL ends with a forward slash
			if !strings.HasSuffix(finalURL, "/") {
				finalURL += "/"
			}

			finalURL += sha256
			if extension != "" {
				finalURL += "." + extension
			}
		}

		return finalURL, statusCode, nil
	}
}
