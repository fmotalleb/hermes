package templates

import "net/url"

func zonePathSegment(zone string) string {
	return url.PathEscape(zone)
}

func recordPathSegment(id string) string {
	return url.PathEscape(id)
}

func zoneDOMID(zone string) string {
	return url.QueryEscape(zone)
}
