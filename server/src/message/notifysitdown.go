package message

import (
    "local/hansa/simple"
)

type NotifySitdownData struct {
    Identity simple.Identity
    Index int
    Sitdown bool
}

