package common

// UserIDKey is the key used in fiber.Ctx.Locals to store the authenticated
// user's ID, set by the auth middleware and read by the handlers of the
// protected modules.
const UserIDKey = "user_id"
