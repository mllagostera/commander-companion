/**
 * Logs the Node.js version once per server instance (each cold start on
 * Vercel), so the runtime's patch level can be read from the logs — the
 * project only pins `24.x`, and the build log doesn't print the patch.
 */
export default defineNitroPlugin(() => {
  console.info(`[runtime] Node.js ${process.version}`)
})
