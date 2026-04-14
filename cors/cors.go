package cors

// DefaultAllowedHeaders is the base set of HTTP headers that all services
// should accept in CORS preflight requests. Services may extend this list
// with service-specific headers using append().
var DefaultAllowedHeaders = []string{
	"Content-Type",
	"Authorization",
	"X-Client-Source",
}
