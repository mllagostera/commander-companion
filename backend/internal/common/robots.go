package common

import "github.com/gofiber/fiber/v2"

// robotsTxt keeps every crawler off the API host. Nothing it serves is meant
// for a search index: JSON responses, auth endpoints and /health. The public
// site, with its own robots.txt and sitemap, lives on the web host.
const robotsTxt = "User-agent: *\nDisallow: /\n"

// RegisterRobotsRoute registers GET /robots.txt at the root, next to /health
// and outside /api/v1, which is where crawlers look for it.
func RegisterRobotsRoute(app *fiber.App) {
	app.Get("/robots.txt", func(c *fiber.Ctx) error {
		c.Type("txt", "utf-8")
		return c.SendString(robotsTxt)
	})
}

// NoIndex marks every response with `X-Robots-Tag: noindex, nofollow`. The
// robots.txt above stops well-behaved crawlers from fetching anything, but a
// URL linked from elsewhere can still be indexed without being crawled; the
// header covers the crawlers that do fetch, and errors too, since Fiber's
// ErrorHandler writes on the same context.
func NoIndex(c *fiber.Ctx) error {
	c.Set("X-Robots-Tag", "noindex, nofollow")
	return c.Next()
}
