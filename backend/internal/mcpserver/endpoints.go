package mcpserver

// The REST endpoints the MCP server calls, relative to the API base URL
// (".../api/v1"). They are the only paths this package may build requests
// against: .github/scripts/check-architecture.sh checks every one of them
// against docs/api/openapi.yaml, so a path that leaves the contract fails CI
// here instead of surfacing as a 404 in a chat. Keep one `pathXxx = "..."`
// constant per line, which is the shape the script greps for.
const (
	pathLogin   = "/auth/login"
	pathRefresh = "/auth/refresh"
	pathLogout  = "/auth/logout"

	pathDecks = "/decks"
	pathDeck  = "/decks/{id}"

	pathPlaygroups = "/playgroups"
	pathPlaygroup  = "/playgroups/{id}"

	pathGame         = "/games/{id}"
	pathGameTimeline = "/games/{id}/timeline"

	pathDashboard          = "/statistics/dashboard"
	pathDeckStatistics     = "/statistics/decks"
	pathOneDeckStatistics  = "/statistics/deck/{id}"
	pathPlaygroupStats     = "/statistics/playgroup/{id}"
	pathPlaygroupGameCount = "/statistics/playgroups"
	pathOpponents          = "/statistics/opponents"
	pathFinishedGames      = "/statistics/games"
	pathBreakdown          = "/statistics/breakdown"

	pathTournaments      = "/tournaments"
	pathTournament       = "/tournaments/{id}"
	pathTournamentLookup = "/tournaments/lookup"
)
