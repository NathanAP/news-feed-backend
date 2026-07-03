package schemas

// DevUserGoogleID is the fixed google_id of the development seed user. It is the shared identity
// between the seed commands (cmd/seed, which create the user) and the dev-login endpoint (which logs
// it in without OAuth). Using a fixed google_id makes the dev user unambiguous and the seed
// idempotent (create-if-absent). Development environment only.
const DevUserGoogleID = "dev-seed-user"
