package proton

import (
	//"context"
	//"log/slog"
	//"sync"

	"github.com/ProtonMail/go-proton-api"
	//"github.com/ProtonMail/gopenpgp/v2/crypto"
)

type Link struct {
	Name string

	Type *proton.LinkType

	XAttr *proton.RevisionXAttrCommon

	Size *int64

	ModifyTime     *int64
	CreateTime     *int64
	ExpirationTime *int64

	session *Session
	//volume  *Volume
	pLink *proton.Link
	//share   *Share
}
