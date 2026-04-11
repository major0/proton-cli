module github.com/major0/proton-cli/smoke

go 1.26.1

require (
	github.com/major0/proton-cli v0.0.0
	github.com/spf13/cobra v1.8.1
)

require (
	github.com/ProtonMail/bcrypt v0.0.0-20211005172633-e235017c1baf // indirect
	github.com/ProtonMail/gluon v0.17.1-0.20260225115619-c0f05c033a4a // indirect
	github.com/ProtonMail/go-crypto v1.4.1-proton // indirect
	github.com/ProtonMail/go-mime v0.0.0-20230322103455-7d82a3887f2f // indirect
	github.com/ProtonMail/go-proton-api v0.4.0 // indirect
	github.com/ProtonMail/go-srp v0.0.7 // indirect
	github.com/ProtonMail/gopenpgp/v2 v2.10.0-proton // indirect
	github.com/PuerkitoBio/goquery v1.12.0 // indirect
	github.com/andybalholm/cascadia v1.3.3 // indirect
	github.com/bradenaw/juniper v0.15.3 // indirect
	github.com/cloudflare/circl v1.6.2 // indirect
	github.com/cronokirby/saferith v0.33.0 // indirect
	github.com/danieljoos/wincred v1.2.3 // indirect
	github.com/emersion/go-message v0.18.2 // indirect
	github.com/emersion/go-vcard v0.0.0-20241024213814-c9703dde27ff // indirect
	github.com/go-resty/resty/v2 v2.17.2 // indirect
	github.com/godbus/dbus/v5 v5.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/major0/optargs v0.4.2 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/sirupsen/logrus v1.9.4 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/zalando/go-keyring v0.2.8 // indirect
	gitlab.com/c0b/go-ordered-json v0.0.0-20201030195603-febf46534d5a // indirect
	golang.org/x/crypto v0.49.0 // indirect
	golang.org/x/exp v0.0.0-20260312153236-7ab1446f8b90 // indirect
	golang.org/x/net v0.52.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/term v0.41.0 // indirect
	golang.org/x/text v0.36.0 // indirect
)

replace (
	github.com/ProtonMail/go-crypto => github.com/ProtonMail/go-crypto v1.1.5-proton
	github.com/ProtonMail/go-proton-api => github.com/major0/go-proton-api v0.0.5-proton-cli
	github.com/ProtonMail/gopenpgp/v2 => github.com/ProtonMail/gopenpgp/v2 v2.10.0-proton
	github.com/go-resty/resty/v2 => github.com/LBeernaertProton/resty/v2 v2.0.0-20231129100320-dddf8030d93a
	github.com/major0/proton-cli => ../
	github.com/spf13/pflag => github.com/major0/optargs/pflag v0.5.0
)
