package simple

// Implemented by Ipmap in the server package, this allows table and sng to
// check if two identities are coming from the same IP.
type IpChecker interface {
    Add(Identity, string)
    Sub(Identity, string)
    Use(string, Identity) (Identity, bool)
    DoneUse(string, Identity)
}

type NoopIpChecker struct {}
func NewNoopIpChecker() *NoopIpChecker {
    return &NoopIpChecker{}
}
func (n *NoopIpChecker) Add(i Identity, ip string) {}
func (n *NoopIpChecker) Sub(i Identity, ip string) {}
func (n *NoopIpChecker) DoneUse(ip string, i Identity) {}
func (n *NoopIpChecker) Use(ip string, i Identity) (Identity, bool) {
    return EmptyIdentity, true
}
