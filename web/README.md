# Commander Companion — Web (Nuxt)

Cliente web, desacoplado del backend (solo consume la API REST vía HTTP, ver
`docs/api/openapi.yaml`). Ver [ADR-0004](../docs/decisions/0004-web-client-nuxt.md)
para el contexto de la decisión.

Estado actual: esqueleto básico con login (email/password + Google Sign-In) y
una pantalla protegida mínima. El resto de las features (import de decks,
estadísticas) todavía no está.

## Stack

- Nuxt 4 (SSR), Tailwind CSS (`@nuxtjs/tailwindcss`), npm.

## Setup

```bash
cd web
npm install
cp .env.example .env   # completar NUXT_PUBLIC_API_BASE / NUXT_PUBLIC_GOOGLE_CLIENT_ID
npm run dev            # http://localhost:3000
```

Necesita el backend corriendo (ver `backend/README` / `backend/docker-compose.yml`)
y `CORS_ALLOWED_ORIGINS` en el backend debe incluir el origin de este cliente
(o dejarlo vacío en dev).

Para que el botón de Google funcione, en Google Cloud Console → **Credentials**
→ el Web Application OAuth Client → **Authorized JavaScript origins**, agregá
el origin donde corre este cliente (ej. `http://localhost:3000`).

## Estructura

```
web/
└── app/                  # srcDir de Nuxt 4 — todo el código de la app vive acá
    ├── pages/
    │   ├── login.vue     # email/password + botón de Google Sign-In
    │   └── index.vue     # pantalla protegida, muestra el usuario autenticado
    ├── composables/
    │   ├── useAuth.ts            # login/loginWithGoogle/logout/fetchMe, sesión en cookies
    │   └── useGoogleIdentity.ts  # carga el script de Google Identity Services y renderiza el botón
    ├── middleware/
    │   └── auth.global.ts        # redirige a /login sin sesión, y fuera de /login con sesión
    └── types/
        └── google-identity.d.ts  # tipado mínimo de window.google
```

## Notas

- La sesión (access/refresh token) se guarda en cookies (`useCookie`, no
  `httpOnly` todavía) para que funcione tanto en SSR como en el cliente. Es
  un punto a endurecer más adelante si se necesita mitigar XSS.
- No hay lógica compartida con `tools/auth-test/` (esa es una herramienta de
  desarrollo standalone, no un cliente real) ni con el cliente Android — cada
  uno implementa el mismo contrato REST por su cuenta.
