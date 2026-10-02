package deck

import "regexp"

var figmaWeb = regexp.MustCompile(`^https://(?:www\.)?figma\.com/`)

// DesktopURL is the Figma desktop app's equivalent of a Figma link, or "" for
// any other link. Figma registers the figma:// scheme and keeps the path and
// query, so the node-id still lands in the right frame.
func (l Link) DesktopURL() string {
	if l.Kind != LinkFigma || !figmaWeb.MatchString(l.URL) {
		return ""
	}
	return figmaWeb.ReplaceAllString(l.URL, "figma://")
}
