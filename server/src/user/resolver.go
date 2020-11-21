package user
// Break dependency cycle on table package, this is TableServer.
type TableResolver interface {
    GetName(int) string
}
