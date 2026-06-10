// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package constants

// ContextKey нь request context-д утга хадгалах typed key юм. Энгийн
// string key ашиглавал `go vet` гомдох (мөн package хооронд мөргөлдөх)
// тул typed төрөл ашиглана.
type ContextKey string

// CtxAuthenticatedUserKey нь auth middleware-ийн доод урсгалын
// handler-уудад зориулж баталгаажуулсан JWT claim-г хадгалдаг request
// context-ийн key юм.
const CtxAuthenticatedUserKey ContextKey = "CtxAuthenticatedUserKey"
