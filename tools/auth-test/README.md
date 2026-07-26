# Auth Test — herramienta de desarrollo

Página HTML de un solo archivo, sin build ni dependencias, para probar el
flujo de autenticación (`internal/auth`) contra una instancia real del
backend, incluido el login con Google (que no se puede probar con `curl`
porque hace falta un `id_token` real emitido por Google en el navegador).

**No es parte del producto.** No la levantes en producción ni la vincules
desde la app Android/web real.

## Cómo usarla

1. Levantá el backend (con Postgres migrado) y anotá su URL, por ejemplo
   `http://localhost:8080`. Ver `backend/README` / `backend/docker-compose.yml`.

2. Servila con cualquier servidor estático simple (Google Identity Services
   no funciona abriendo el archivo directo con `file://`, necesita un origin
   `http://` o `https://`):

   ```bash
   # Opción A: Python (ya viene instalado en la mayoría de los sistemas)
   cd tools/auth-test
   python3 -m http.server 5500

   # Opción B: Node
   npx --yes serve tools/auth-test -l 5500
   ```

   Abrí `http://localhost:5500` en el navegador.

3. En el backend, asegurate de que `CORS_ALLOWED_ORIGINS` incluya el origin
   donde serviste esta página (o dejalo vacío en dev, que por defecto permite
   cualquier origin — ver `backend/.env.example`).

4. Para probar el login con Google, en Google Cloud Console → **Credentials**
   → tu **Web application** OAuth Client → **Authorized JavaScript origins**,
   agregá el origin exacto donde serviste esta página (ej.
   `http://localhost:5500`). Sin esto, el botón de Google va a fallar.

5. En la página:
   - Pegá el **API base URL** (con `/api/v1` al final) y el **Google Web
     Client ID** (el mismo valor que `GOOGLE_CLIENT_ID` en el backend) y
     tocá "Guardar y (re)cargar botón de Google". Ambos quedan guardados en
     `localStorage` para la próxima vez.
   - Registrate y logueate con email/password, o usá el botón de Google.
   - Con la sesión activa, probá `GET /auth/me`, `GET /decks` (ruta
     protegida — confirma que el middleware JWT deja pasar el request),
     `POST /auth/refresh` (rota el refresh token) y `POST /auth/logout`.
   - El panel de la derecha muestra cada request/response tal cual los
     devuelve la API.

## Qué confirma este flujo

- Que `/auth/register` y `/auth/login` funcionan de punta a punta contra
  Postgres (no dummy).
- Que el middleware `RequireAuth` efectivamente bloquea rutas sin token y
  las deja pasar con uno válido.
- Que `/auth/refresh` rota el refresh token (el anterior queda revocado).
- Que `/auth/google` verifica un `id_token` real emitido por Google contra
  el `GOOGLE_CLIENT_ID` configurado, y crea o vincula la cuenta
  correspondiente.
