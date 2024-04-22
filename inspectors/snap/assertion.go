// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2024 Canonical Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package snap

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
	"sync"

	"github.com/ProtonMail/go-crypto/openpgp/packet"

	"github.com/canonical/fetch-service/logger"
	"github.com/canonical/fetch-service/utils"
)

var (
	CanonicalAccount        *assertion
	CanonicalRootAccountKey *assertion
	CanonicalPublicKey      *packet.PublicKey

	nlnl = []byte("\n\n")

	accountKeyCache      = map[string]*packet.PublicKey{}
	accountKeyCacheMutex sync.Mutex
)

func init() {
	var err error
	CanonicalAccount, err = newAssertion([]byte(encodedCanonicalAccount))
	if err != nil {
		panic(fmt.Sprintf("cannot decode trusted assertion: %v", err))
	}
	CanonicalRootAccountKey, err = newAssertion([]byte(encodedCanonicalRootAccountKey))
	if err != nil {
		panic(fmt.Sprintf("cannot decode trusted assertion: %v", err))
	}
	CanonicalPublicKey, err = decodePublicKey(CanonicalRootAccountKey.Body)
	if err != nil {
		panic(fmt.Sprintf("cannot decode canonical public key: %v", err))
	}

}

const (
	encodedCanonicalAccount = `type: account
authority-id: canonical
account-id: canonical
display-name: Canonical
timestamp: 2016-04-01T00:00:00.0Z
username: canonical
validation: certified
sign-key-sha3-384: -CvQKAwRQ5h3Ffn10FILJoEZUXOv6km9FwA80-Rcj-f-6jadQ89VRswHNiEB9Lxk

AcLDXAQAAQoABgUCV7UYzwAKCRDUpVvql9g3IK7uH/4udqNOurx5WYVknzXdwekp0ovHCQJ0iBPw
TSFxEVr9faZSzb7eqJ1WicHsShf97PYS3ClRYAiluFsjRA8Y03kkSVJHjC+sIwGFubsnkmgflt6D
WEmYIl0UBmeaEDS8uY4Xvp9NsLTzNEj2kvzy/52gKaTc1ZSl5RDL9ppMav+0V9iBYpiDPBWH2rJ+
aDSD8Rkyygm0UscfAKyDKH4lrvZ0WkYyi1YVNPrjQ/AtBySh6Q4iJ3LifzKa9woIyAuJET/4/FPY
oirqHAfuvNod36yNQIyNqEc20AvTvZNH0PSsg4rq3DLjIPzv5KbJO9lhsasNJK1OdL6x8Yqrdsbk
ldZp4qkzfjV7VOMQKaadfcZPRaVVeJWOBnBiaukzkhoNlQi1sdCdkBB/AJHZF8QXw6c7vPDcfnCV
1lW7ddQ2p8IsJbT6LzpJu3GW/P4xhNgCjtCJ1AJm9a9RqLwQYgdLZwwDa9iCRtqTbRXBlfy3apps
1VjbQ3h5iCd0hNfwDBnGVm1rhLKHCD1DUdNE43oN2ZlE7XGyh0HFV6vKlpqoW3eoXCIxWu+HBY96
+LSl/jQgCkb0nxYyzEYK4Reb31D0mYw1Nji5W+MIF5E09+DYZoOT0UvR05YMwMEOeSdI/hLWg/5P
k+GDK+/KopMmpd4D1+jjtF7ZvqDpmAV98jJGB2F88RyVb4gcjmFFyTi4Kv6vzz/oLpbm0qrizC0W
HLGDN/ymGA5sHzEgEx7U540vz/q9VX60FKqL2YZr/DcyY9GKX5kCG4sNqIIHbcJneZ4frM99oVDu
7Jv+DIx/Di6D1ULXol2XjxbbJLKHFtHksR97ceaFvcZwTogC61IYUBJCvvMoqdXAWMhEXCr0QfQ5
Xbi31XW2d4/lF/zWlAkRnGTzufIXFni7+nEuOK0SQEzO3/WaRedK1SGOOtTDjB8/3OJeW96AUYK5
oTIynkYkEyHWMNCXALg+WQW6L4/YO7aUjZ97zOWIugd7Xy63aT3r/EHafqaY2nacOhLfkeKZ830b
o/ezjoZQAxbh6ce7JnXRgE9ELxjdAhBTpGjmmmN2sYrJ7zP9bOgly0BnEPXGSQfFA+NNNw1FADx1
MUY8q9DBjmVtgqY+1KGTV5X8KvQCBMODZIf/XJPHdCRAHxMd8COypcwgL2vDIIXpOFbi1J/B0GF+
eklxk9wzBA8AecBMCwCzIRHDNpD1oa2we38bVFrOug6e/VId1k1jYFJjiLyLCDmV8IMYwEllHSXp
LQAdm3xZ7t4WnxYC8YSCk9mXf3CZg59SpmnV5Q5Z6A5Pl7Nc3sj7hcsMBZEsOMPzNC9dPsBnZvjs
WpPUffJzEdhHBFhvYMuD4Vqj6ejUv9l3oTrjQWVC
`

	encodedCanonicalRootAccountKey = `type: account-key
authority-id: canonical
revision: 2
public-key-sha3-384: -CvQKAwRQ5h3Ffn10FILJoEZUXOv6km9FwA80-Rcj-f-6jadQ89VRswHNiEB9Lxk
account-id: canonical
name: root
since: 2016-04-01T00:00:00.0Z
body-length: 1406
sign-key-sha3-384: -CvQKAwRQ5h3Ffn10FILJoEZUXOv6km9FwA80-Rcj-f-6jadQ89VRswHNiEB9Lxk

AcbDTQRWhcGAASAA4Zdo3CVpKmTecjd3VDBiFbZTKKhcG0UV3FXxyGIe2UsdnJIks4NkVYO+qYk0
zW26Svpa5OIOJGO2NcgN9bpCYWZOufO1xTmC7jW/fEtqJpX8Kcq20+X5AarqJ5RBVnGLrlz+ZT99
aHdRZ4YQ2XUZvhbelzWTdK5+2eMSXNrFjO6WwGh9NRekE/NIBNwvULAtJ5nv1KwZaSpZ+klJrstU
EHPhs+NGGm1Aru01FFl3cWUm5Ao8i9y+pFcPoaRatgtpYU8mg9gP594lvyJqjFofXvHPwztmySqf
FVAp4gLLfLvRxbXkOfPUz8guidqvg6r4DUD+kCBjKYoT44PjK6l51MzEL2IEy6jdnFTgjHbaYML8
/5NpuPu8XiSjCpOTeNR+XKzXC2tHRU7j09Xd44vKRhPk0Hc4XsPNBWqfrcbdWmwsFhjfxFDJajOq
hzWVoiRc5opB5socbRjLf+gYtncxe99oC2FDA2FcftlFoyztho0bAzeFer1IHJIMYWxKMESjvJUE
pnMMKpIMYY0QfWEo5hXR0TaT+NxW2Z9Jqclgyw13y5iY72ZparHS66J+C7dxCEOswlw1ypNic6MM
/OzpafIQ10yAT3HeRCJQQOOSSTaold+WpWsQweYCywPcu9S+wCo6CrPzJCCIxOAnXjLYv2ykTJje
pNJ2+GZ1WH2UeJdJ5sR8fpxxRupqHuEKNRZ+2CqLmFC5kHNszoGolLEvGcK4BJciO4KihnKtxrdX
dUJIOPBLktA8XiiHSOmLzs2CFjcvlDuPSpe64HIL5yCxO1/GRux4A1Kht1+DqTrL7DjyIW+vIPro
A1PQwkcAJyScNRxT4bPpUj8geAXWd3n212W+7QVHuQEFezvXC5GbMyR+Xj47FOFcFcSZID1hTZEu
uMD+AxaBHQKwPfBx1arVKE1OhkuKHeSFtZRP8K8l3qj5W0sIxxIW19W8aziu8ZeDMT+nIEJrJvhx
zGEdxwCrp3k2/93oDV7g+nb1ZGfIhtmcrKziijghzPLaYaiM9LggqwTARelk3xSzd8+uk3LPXuVl
fP8/xHApss6sCE3xk4+F3OGbL7HbGuCnoulf795XKLRTy+xU/78piOMNJJQu+G0lMZIO3cZrP6io
MYDa+jDZw4V4fBRWce/FA3Ot1eIDxCq5v+vfKw+HfUlWcjm6VUQIFZYbK+Lzj6mpXn81BugG3d+M
0WNFObXIrUbhnKcYkus3TSJ9M1oMEIMp0WfFGAVTd61u36fdi2e+/xbLN0kbYcFRZwd9CmtEeDZ0
eYx/pvKKaNz/DfUr0piVCRwxuxQ0kVppklHPO4sOTFZUId8KLHg28LbszvupSsHP/nHlW8l5/VK6
4+KxRV2XofsUnwARAQAB

AcLDXAQAAQoABgUCV83kkgAKCRDUpVvql9g3IA9hIADAkn4VXnJIFblhMSBe6hbTy7z6AfOhZxXR
Ds/mHsiWfFT6ifGi9SpZowhRX+ff57YvFCjlBqMYLKYE0NsFQYEUc5uBWiFZwC0ENydNhO23DV1B
elTSs6mr9duPm1eJAozFrQETOD1kz5BIamqBUeaTczjM+9l5i485Ffknbc+EaGOrtMEap0GqjByQ
u+ykZGvryVQ447avgjvFsMtA0quFi+SoW9PT/9D26e5rD7RIICYWG8mzFRn5Isqs/X4W1uAiKQe9
pqHMbdNr/FCWX5ws0/nMaOq+b0z4EIIXIfT0JmIlFDQsAgFVnKwYw+zs32cTw4XuzvMhgMDtCowD
YodhiO/5AOMsMMV0qBsYxbIPJIEz7b6gwTYEJoTVkqTit6o3UgWrAy+p4Y7t0ickYIHgwiuKRS9E
fu0Ue+32NFp0XFqZElfXLK/U2yjto+fJXu6uAELsXesfFGIOp/nbRbNavUt9jAJeO7ftQczgf39T
YfA0OKerP5gAOd4+aO3gATPUjfWPsJ9908XC7QqK2BwS1kh/fMrd95mxcmXdF1bBElszKwaToBVQ
1m52EYp06kkPyOu+fGKFAoIMafcV/2Ztz1WMo/Vp0iP/r0WAtBDw6sDJyWOfRjUEvP7BBdEzraHV
VblbSrKzhYeEGdMDi6kFC+KEzfPDPFJX1l3saPBkz9VDuESbktyObQp9VfkFKYBgBnw3msQJk+6k
G4t0o3/DZ7qz/kTJXMogG26Z/FsMhPERsaLTbWRJ3WRyXX8COaTladSf8bG0Oib19outnjuvpjQ0
qEV9eeGRBlx9mbidSYH95cj0zD2DKpeSZ83M5K1pFg+8RKToGElGTTk8vtdTfDVbmi3+QntfLq+z
ZMgs2+SmCWrV/MPC04Dl00CXywdKPyf6toomqRP7A5fS7W8P9fdPn+a8JCblcleGj9nvJXBQjue7
97rofCEszhKhoE9fMCIUcSoTU9YAm5Jr+qclSEbV1pzwTvZ8auMIXtzEZV5n4aK4WPDV+lYCadrL
DlvJSJRuXRvIMbmvU9b8NxgG8AS88BkX3L9vlOpkMculwG1/iooQvxuFaJDargt370wAQo0lCpG3
MxnsSusymwnYegvvvr7Xp/KBLZK1+8Djzm3fwAryp4qNo29ciVw3O9lFKmmuiIcxSY0bauXaK6kv
pTnYkmx7XGPF7Ahb7Ov0/0FE2Lx3JZXSEKeW+VrCcpYQOY++t67b+jf0AV4rZExcLFJzP6MPMimP
ZCd383NzlzkXK+vAdvTi40HPiM9FYOp6g8JTs5TTdx2/qs/SWFC8AkahIQmH0IpFBJep2JKl2kyr
FZMvASkHA9bR/UuXDvbMzsUmT/xnERZosQaZgFEO
`
)

type assertion struct {
	Header    map[string]string
	Body      []byte
	Content   []byte
	Signature []byte
}

func newAssertion(serializedAssertion []byte) (*assertion, error) {
	// copy to get an independent backstorage that can't be mutated later
	assertionSnapshot := make([]byte, len(serializedAssertion))
	copy(assertionSnapshot, serializedAssertion)

	contentSignatureSplit := bytes.LastIndex(assertionSnapshot, nlnl)

	if contentSignatureSplit == -1 {
		return nil, fmt.Errorf("assertion content/signature separator not found")
	}

	content := assertionSnapshot[:contentSignatureSplit]
	signature := assertionSnapshot[contentSignatureSplit+2:]

	headersBodySplit := bytes.Index(content, nlnl)
	var body, head []byte
	if headersBodySplit == -1 {
		head = content
	} else {
		body = content[headersBodySplit+2:]
		if len(body) == 0 {
			body = nil
		}
		head = content[:headersBodySplit]
	}

	headers, err := parseHeaders(head)
	if err != nil {
		return nil, fmt.Errorf("parsing assertion headers: %v", err)
	}

	return &assertion{
		Header:    headers,
		Body:      body,
		Content:   content,
		Signature: signature,
	}, nil
}

// parseHeaders is a simplified version of the snapd assertion
// header parser, restricted to single-line entries. See
// snapd/asserts/headers.go for further information.
func parseHeaders(head []byte) (map[string]string, error) {
	sc := bufio.NewScanner(bytes.NewReader(head))
	sc.Split(bufio.ScanLines)

	header := map[string]string{}

	// Parse header
	for sc.Scan() {
		line := sc.Text()
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			break
		}
		v = strings.TrimSpace(v)
		header[k] = v
	}

	return header, nil
}

// VerifySignature checks the integrity of the assertion's
// signed content.
func (assert assertion) VerifySignature() error {
	signature, err := decodeSignature(assert.Signature)
	if err != nil {
		return err
	}

	signKey := assert.SignKey()
	if signKey == "" {
		return fmt.Errorf("cannot find sign-key-sha3-384 in snap-revision assertion")
	}

	accountKeyCacheMutex.Lock()
	cachedKey, ok := accountKeyCache[signKey]
	accountKeyCacheMutex.Unlock()

	if ok {
		// Cached keys were already verified.
		logger.Infof("account key %s is cached", signKey)
		return utils.VerifySignature(cachedKey, signature, assert.Content)
	}

	logger.Debugf("verify assertion signature %s", signKey)

	if signKey == CanonicalRootAccountKey.SignKey() {
		return utils.VerifySignature(CanonicalPublicKey, signature, assert.Content)
	}

	// Retrieve signing account key
	accountKeyAssertion, err := downloadAccountKeyAssertion(signKey)
	if err != nil {
		return fmt.Errorf("cannot retrieve account-key assertion: %w", err)
	}

	if err := accountKeyAssertion.VerifySignature(); err != nil {
		return err
	}

	key, err := decodePublicKey(accountKeyAssertion.Body)
	if err != nil {
		return fmt.Errorf("cannot decode public key: %w", err)
	}

	// The signature is verified, add to cache
	accountKeyCacheMutex.Lock()
	accountKeyCache[signKey] = key
	accountKeyCacheMutex.Unlock()

	logger.Infof("account key %s verified", signKey)

	return utils.VerifySignature(key, signature, assert.Content)
}

func (assert assertion) SnapID() string {
	return assert.Header["snap-id"]
}

func (assert assertion) PublisherID() string {
	return assert.Header["publisher-id"]
}

func (assert assertion) SnapName() string {
	return assert.Header["snap-name"]
}

func (assert assertion) SnapSize() string {
	return assert.Header["snap-size"]
}

func (assert assertion) SnapSha384() string {
	return assert.Header["snap-sha3-384"]
}

func (assert assertion) SnapRevision() string {
	return assert.Header["snap-revision"]
}

func (assert assertion) DisplayName() string {
	return assert.Header["display-name"]
}

func (assert assertion) Type() string {
	return assert.Header["type"]
}

func (assert assertion) AuthorityID() string {
	return assert.Header["authority-id"]
}

func (assert assertion) SignKey() string {
	return assert.Header["sign-key-sha3-384"]
}
