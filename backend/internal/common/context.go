package common

// UserIDKey es la clave usada en fiber.Ctx.Locals para guardar el ID del
// usuario autenticado, seteada por el middleware de auth y leída por los
// handlers de los módulos protegidos.
const UserIDKey = "user_id"
