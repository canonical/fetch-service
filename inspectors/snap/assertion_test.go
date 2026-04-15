// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright 2024-2025 Canonical Ltd.
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

package snap_test

import (
	. "gopkg.in/check.v1"

	"github.com/canonical/fetch-service/inspectors/snap"
)

func (t *snapSuite) TestAccountAssertion(c *C) {
	c.Assert(snap.CanonicalAccount.Type(), Equals, "account")
	c.Assert(snap.CanonicalAccount.AuthorityID(), Equals, "canonical")
	c.Assert(snap.CanonicalAccount.DisplayName(), Equals, "Canonical")
	c.Assert(snap.CanonicalAccount.SnapID(), Equals, "")
	c.Assert(snap.CanonicalAccount.Header, DeepEquals, map[string]string{
		"type":              "account",
		"authority-id":      "canonical",
		"account-id":        "canonical",
		"display-name":      "Canonical",
		"timestamp":         "2016-04-01T00:00:00.0Z",
		"username":          "canonical",
		"validation":        "certified",
		"sign-key-sha3-384": "-CvQKAwRQ5h3Ffn10FILJoEZUXOv6km9FwA80-Rcj-f-6jadQ89VRswHNiEB9Lxk",
	})
	c.Assert(snap.CanonicalAccount.Body, IsNil)
	c.Assert(snap.CanonicalAccount.Signature, DeepEquals, []byte(""+
		"AcLDXAQAAQoABgUCV7UYzwAKCRDUpVvql9g3IK7uH/4udqNOurx5WYVknzXdwekp0ovHCQJ0iBPw\n"+
		"TSFxEVr9faZSzb7eqJ1WicHsShf97PYS3ClRYAiluFsjRA8Y03kkSVJHjC+sIwGFubsnkmgflt6D\n"+
		"WEmYIl0UBmeaEDS8uY4Xvp9NsLTzNEj2kvzy/52gKaTc1ZSl5RDL9ppMav+0V9iBYpiDPBWH2rJ+\n"+
		"aDSD8Rkyygm0UscfAKyDKH4lrvZ0WkYyi1YVNPrjQ/AtBySh6Q4iJ3LifzKa9woIyAuJET/4/FPY\n"+
		"oirqHAfuvNod36yNQIyNqEc20AvTvZNH0PSsg4rq3DLjIPzv5KbJO9lhsasNJK1OdL6x8Yqrdsbk\n"+
		"ldZp4qkzfjV7VOMQKaadfcZPRaVVeJWOBnBiaukzkhoNlQi1sdCdkBB/AJHZF8QXw6c7vPDcfnCV\n"+
		"1lW7ddQ2p8IsJbT6LzpJu3GW/P4xhNgCjtCJ1AJm9a9RqLwQYgdLZwwDa9iCRtqTbRXBlfy3apps\n"+
		"1VjbQ3h5iCd0hNfwDBnGVm1rhLKHCD1DUdNE43oN2ZlE7XGyh0HFV6vKlpqoW3eoXCIxWu+HBY96\n"+
		"+LSl/jQgCkb0nxYyzEYK4Reb31D0mYw1Nji5W+MIF5E09+DYZoOT0UvR05YMwMEOeSdI/hLWg/5P\n"+
		"k+GDK+/KopMmpd4D1+jjtF7ZvqDpmAV98jJGB2F88RyVb4gcjmFFyTi4Kv6vzz/oLpbm0qrizC0W\n"+
		"HLGDN/ymGA5sHzEgEx7U540vz/q9VX60FKqL2YZr/DcyY9GKX5kCG4sNqIIHbcJneZ4frM99oVDu\n"+
		"7Jv+DIx/Di6D1ULXol2XjxbbJLKHFtHksR97ceaFvcZwTogC61IYUBJCvvMoqdXAWMhEXCr0QfQ5\n"+
		"Xbi31XW2d4/lF/zWlAkRnGTzufIXFni7+nEuOK0SQEzO3/WaRedK1SGOOtTDjB8/3OJeW96AUYK5\n"+
		"oTIynkYkEyHWMNCXALg+WQW6L4/YO7aUjZ97zOWIugd7Xy63aT3r/EHafqaY2nacOhLfkeKZ830b\n"+
		"o/ezjoZQAxbh6ce7JnXRgE9ELxjdAhBTpGjmmmN2sYrJ7zP9bOgly0BnEPXGSQfFA+NNNw1FADx1\n"+
		"MUY8q9DBjmVtgqY+1KGTV5X8KvQCBMODZIf/XJPHdCRAHxMd8COypcwgL2vDIIXpOFbi1J/B0GF+\n"+
		"eklxk9wzBA8AecBMCwCzIRHDNpD1oa2we38bVFrOug6e/VId1k1jYFJjiLyLCDmV8IMYwEllHSXp\n"+
		"LQAdm3xZ7t4WnxYC8YSCk9mXf3CZg59SpmnV5Q5Z6A5Pl7Nc3sj7hcsMBZEsOMPzNC9dPsBnZvjs\n"+
		"WpPUffJzEdhHBFhvYMuD4Vqj6ejUv9l3oTrjQWVC\n"))
	c.Assert(snap.CanonicalAccount.Content, DeepEquals, []byte(""+
		"type: account\n"+
		"authority-id: canonical\n"+
		"account-id: canonical\n"+
		"display-name: Canonical\n"+
		"timestamp: 2016-04-01T00:00:00.0Z\n"+
		"username: canonical\n"+
		"validation: certified\n"+
		"sign-key-sha3-384: -CvQKAwRQ5h3Ffn10FILJoEZUXOv6km9FwA80-Rcj-f-6jadQ89VRswHNiEB9Lxk"))

	err := snap.CanonicalAccount.VerifySignature(t.sl)
	c.Assert(err, IsNil)
}

func (t *snapSuite) TestAccountKeyAssertion(c *C) {
	c.Assert(snap.CanonicalRootAccountKey.Type(), Equals, "account-key")
	c.Assert(snap.CanonicalRootAccountKey.AuthorityID(), Equals, "canonical")
	c.Assert(snap.CanonicalRootAccountKey.DisplayName(), Equals, "")
	c.Assert(snap.CanonicalRootAccountKey.Header, DeepEquals, map[string]string{
		"type":                "account-key",
		"authority-id":        "canonical",
		"revision":            "2",
		"account-id":          "canonical",
		"name":                "root",
		"since":               "2016-04-01T00:00:00.0Z",
		"body-length":         "1406",
		"public-key-sha3-384": "-CvQKAwRQ5h3Ffn10FILJoEZUXOv6km9FwA80-Rcj-f-6jadQ89VRswHNiEB9Lxk",
		"sign-key-sha3-384":   "-CvQKAwRQ5h3Ffn10FILJoEZUXOv6km9FwA80-Rcj-f-6jadQ89VRswHNiEB9Lxk",
	})
	c.Assert(snap.CanonicalRootAccountKey.Body, DeepEquals, []byte(""+
		"AcbDTQRWhcGAASAA4Zdo3CVpKmTecjd3VDBiFbZTKKhcG0UV3FXxyGIe2UsdnJIks4NkVYO+qYk0\n"+
		"zW26Svpa5OIOJGO2NcgN9bpCYWZOufO1xTmC7jW/fEtqJpX8Kcq20+X5AarqJ5RBVnGLrlz+ZT99\n"+
		"aHdRZ4YQ2XUZvhbelzWTdK5+2eMSXNrFjO6WwGh9NRekE/NIBNwvULAtJ5nv1KwZaSpZ+klJrstU\n"+
		"EHPhs+NGGm1Aru01FFl3cWUm5Ao8i9y+pFcPoaRatgtpYU8mg9gP594lvyJqjFofXvHPwztmySqf\n"+
		"FVAp4gLLfLvRxbXkOfPUz8guidqvg6r4DUD+kCBjKYoT44PjK6l51MzEL2IEy6jdnFTgjHbaYML8\n"+
		"/5NpuPu8XiSjCpOTeNR+XKzXC2tHRU7j09Xd44vKRhPk0Hc4XsPNBWqfrcbdWmwsFhjfxFDJajOq\n"+
		"hzWVoiRc5opB5socbRjLf+gYtncxe99oC2FDA2FcftlFoyztho0bAzeFer1IHJIMYWxKMESjvJUE\n"+
		"pnMMKpIMYY0QfWEo5hXR0TaT+NxW2Z9Jqclgyw13y5iY72ZparHS66J+C7dxCEOswlw1ypNic6MM\n"+
		"/OzpafIQ10yAT3HeRCJQQOOSSTaold+WpWsQweYCywPcu9S+wCo6CrPzJCCIxOAnXjLYv2ykTJje\n"+
		"pNJ2+GZ1WH2UeJdJ5sR8fpxxRupqHuEKNRZ+2CqLmFC5kHNszoGolLEvGcK4BJciO4KihnKtxrdX\n"+
		"dUJIOPBLktA8XiiHSOmLzs2CFjcvlDuPSpe64HIL5yCxO1/GRux4A1Kht1+DqTrL7DjyIW+vIPro\n"+
		"A1PQwkcAJyScNRxT4bPpUj8geAXWd3n212W+7QVHuQEFezvXC5GbMyR+Xj47FOFcFcSZID1hTZEu\n"+
		"uMD+AxaBHQKwPfBx1arVKE1OhkuKHeSFtZRP8K8l3qj5W0sIxxIW19W8aziu8ZeDMT+nIEJrJvhx\n"+
		"zGEdxwCrp3k2/93oDV7g+nb1ZGfIhtmcrKziijghzPLaYaiM9LggqwTARelk3xSzd8+uk3LPXuVl\n"+
		"fP8/xHApss6sCE3xk4+F3OGbL7HbGuCnoulf795XKLRTy+xU/78piOMNJJQu+G0lMZIO3cZrP6io\n"+
		"MYDa+jDZw4V4fBRWce/FA3Ot1eIDxCq5v+vfKw+HfUlWcjm6VUQIFZYbK+Lzj6mpXn81BugG3d+M\n"+
		"0WNFObXIrUbhnKcYkus3TSJ9M1oMEIMp0WfFGAVTd61u36fdi2e+/xbLN0kbYcFRZwd9CmtEeDZ0\n"+
		"eYx/pvKKaNz/DfUr0piVCRwxuxQ0kVppklHPO4sOTFZUId8KLHg28LbszvupSsHP/nHlW8l5/VK6\n"+
		"4+KxRV2XofsUnwARAQAB"))
	c.Assert(snap.CanonicalRootAccountKey.Signature, DeepEquals, []byte(""+
		"AcLDXAQAAQoABgUCV83kkgAKCRDUpVvql9g3IA9hIADAkn4VXnJIFblhMSBe6hbTy7z6AfOhZxXR\n"+
		"Ds/mHsiWfFT6ifGi9SpZowhRX+ff57YvFCjlBqMYLKYE0NsFQYEUc5uBWiFZwC0ENydNhO23DV1B\n"+
		"elTSs6mr9duPm1eJAozFrQETOD1kz5BIamqBUeaTczjM+9l5i485Ffknbc+EaGOrtMEap0GqjByQ\n"+
		"u+ykZGvryVQ447avgjvFsMtA0quFi+SoW9PT/9D26e5rD7RIICYWG8mzFRn5Isqs/X4W1uAiKQe9\n"+
		"pqHMbdNr/FCWX5ws0/nMaOq+b0z4EIIXIfT0JmIlFDQsAgFVnKwYw+zs32cTw4XuzvMhgMDtCowD\n"+
		"YodhiO/5AOMsMMV0qBsYxbIPJIEz7b6gwTYEJoTVkqTit6o3UgWrAy+p4Y7t0ickYIHgwiuKRS9E\n"+
		"fu0Ue+32NFp0XFqZElfXLK/U2yjto+fJXu6uAELsXesfFGIOp/nbRbNavUt9jAJeO7ftQczgf39T\n"+
		"YfA0OKerP5gAOd4+aO3gATPUjfWPsJ9908XC7QqK2BwS1kh/fMrd95mxcmXdF1bBElszKwaToBVQ\n"+
		"1m52EYp06kkPyOu+fGKFAoIMafcV/2Ztz1WMo/Vp0iP/r0WAtBDw6sDJyWOfRjUEvP7BBdEzraHV\n"+
		"VblbSrKzhYeEGdMDi6kFC+KEzfPDPFJX1l3saPBkz9VDuESbktyObQp9VfkFKYBgBnw3msQJk+6k\n"+
		"G4t0o3/DZ7qz/kTJXMogG26Z/FsMhPERsaLTbWRJ3WRyXX8COaTladSf8bG0Oib19outnjuvpjQ0\n"+
		"qEV9eeGRBlx9mbidSYH95cj0zD2DKpeSZ83M5K1pFg+8RKToGElGTTk8vtdTfDVbmi3+QntfLq+z\n"+
		"ZMgs2+SmCWrV/MPC04Dl00CXywdKPyf6toomqRP7A5fS7W8P9fdPn+a8JCblcleGj9nvJXBQjue7\n"+
		"97rofCEszhKhoE9fMCIUcSoTU9YAm5Jr+qclSEbV1pzwTvZ8auMIXtzEZV5n4aK4WPDV+lYCadrL\n"+
		"DlvJSJRuXRvIMbmvU9b8NxgG8AS88BkX3L9vlOpkMculwG1/iooQvxuFaJDargt370wAQo0lCpG3\n"+
		"MxnsSusymwnYegvvvr7Xp/KBLZK1+8Djzm3fwAryp4qNo29ciVw3O9lFKmmuiIcxSY0bauXaK6kv\n"+
		"pTnYkmx7XGPF7Ahb7Ov0/0FE2Lx3JZXSEKeW+VrCcpYQOY++t67b+jf0AV4rZExcLFJzP6MPMimP\n"+
		"ZCd383NzlzkXK+vAdvTi40HPiM9FYOp6g8JTs5TTdx2/qs/SWFC8AkahIQmH0IpFBJep2JKl2kyr\n"+
		"FZMvASkHA9bR/UuXDvbMzsUmT/xnERZosQaZgFEO\n"))

	err := snap.CanonicalRootAccountKey.VerifySignature(t.sl)
	c.Assert(err, IsNil)
}

func (t *snapSuite) TestSnapRevisionAssertion(c *C) {
	encodedSnapRevision := `type: snap-revision
authority-id: canonical
snap-sha3-384: Gsqj3QgpWFq2p0517nHNNMZWgX5rG6_vaeOjT9Nyua_l36qkC_HdiDw2iEd4t1J-
developer-id: canonical
provenance: global-upload
snap-id: CSO04Jhav2yK0uz97cr0ipQRyqg0qQL6
snap-revision: 2812
snap-size: 58363904
timestamp: 2023-10-27T17:42:03.206069Z
sign-key-sha3-384: BWDEoaqyr25nF5SNCvEv2v7QnM9QsfCc0PBMYD_i2NGSQ32EF2d4D0hqUel3m8ul

AcLBUgQAAQoABgUCZTv2awAAH3oQAMi877K8gz1Vc9Qhoz73CA/Uozgt2rVcrpoeUeS32aAU+/lO
yaTH05xcZ8ChdlRWYCgH5DCfrNBRPfFWFlaf9QXK/PBZXXCC8f2/5MSt/XU0lh8kAImd4aCCCruB
kOGDee/cg3webCmkt1zdMzDmn55aMMF+63Z3ZX1fNMDESHcHZEV2CMCdnrna05Ap4QloCPcrezMn
p5c93U8t+wZCxQxoo5h+8APs27TuGmLHEPOTgHaNgaFBDiRn8kxSQcdco5NdDBh0e63+keR71i9U
cQRqGF8DGfhrikI+rmMtI7T4mQdI0rc5C73URM/Gzv+UCFZRkox+HrSkoe80JXqPzuX1Kh2kvH/y
/jV20AN9RmOUpNt/tzxDhxmS7N9i9LlY66YAhhVRyW1nZdlwqdo9h7zQ92zcVqCVtFblxUidJGp5
81hoUgZ436fZAzbdAxaFRg5Hb1IJLrnd2g9wWr4tyVUm+4QwTwfwwEpMAgDmwnInjzXgtccgNGFu
4XIEhRhfjPiMArQHVZ4vCIaI/YW+SZ4LZd1SsOmP/7/CX03RIeCgRp9r4OpJYoirUsP1E6HZ++m7
ySzGFuHO82rSnh/QjTO22/nXUMfCGaXSfWWJ+WTnCZvJCaYcHzoldnCBa1AD3FFDej+6N+ry9qpR
Rh44mJJaAnoLgbFiXdyAD/lOOMwt
`
	assert, err := snap.NewAssertion([]byte(encodedSnapRevision))
	c.Assert(err, IsNil)

	c.Assert(assert.Type(), Equals, "snap-revision")
	c.Assert(assert.AuthorityID(), Equals, "canonical")
	c.Assert(assert.SnapID(), Equals, "CSO04Jhav2yK0uz97cr0ipQRyqg0qQL6")
	c.Assert(assert.SnapRevision(), Equals, "2812")
	c.Assert(assert.SnapSha384(), Equals, "Gsqj3QgpWFq2p0517nHNNMZWgX5rG6_vaeOjT9Nyua_l36qkC_HdiDw2iEd4t1J-")
	c.Assert(assert.SnapSize(), Equals, "58363904")
	c.Assert(assert.Header, DeepEquals, map[string]string{
		"type":              "snap-revision",
		"authority-id":      "canonical",
		"snap-sha3-384":     "Gsqj3QgpWFq2p0517nHNNMZWgX5rG6_vaeOjT9Nyua_l36qkC_HdiDw2iEd4t1J-",
		"developer-id":      "canonical",
		"provenance":        "global-upload",
		"snap-id":           "CSO04Jhav2yK0uz97cr0ipQRyqg0qQL6",
		"snap-revision":     "2812",
		"snap-size":         "58363904",
		"timestamp":         "2023-10-27T17:42:03.206069Z",
		"sign-key-sha3-384": "BWDEoaqyr25nF5SNCvEv2v7QnM9QsfCc0PBMYD_i2NGSQ32EF2d4D0hqUel3m8ul",
	})
	c.Assert(assert.Body, IsNil)
	c.Assert(assert.Signature, DeepEquals, []byte(""+
		"AcLBUgQAAQoABgUCZTv2awAAH3oQAMi877K8gz1Vc9Qhoz73CA/Uozgt2rVcrpoeUeS32aAU+/lO\n"+
		"yaTH05xcZ8ChdlRWYCgH5DCfrNBRPfFWFlaf9QXK/PBZXXCC8f2/5MSt/XU0lh8kAImd4aCCCruB\n"+
		"kOGDee/cg3webCmkt1zdMzDmn55aMMF+63Z3ZX1fNMDESHcHZEV2CMCdnrna05Ap4QloCPcrezMn\n"+
		"p5c93U8t+wZCxQxoo5h+8APs27TuGmLHEPOTgHaNgaFBDiRn8kxSQcdco5NdDBh0e63+keR71i9U\n"+
		"cQRqGF8DGfhrikI+rmMtI7T4mQdI0rc5C73URM/Gzv+UCFZRkox+HrSkoe80JXqPzuX1Kh2kvH/y\n"+
		"/jV20AN9RmOUpNt/tzxDhxmS7N9i9LlY66YAhhVRyW1nZdlwqdo9h7zQ92zcVqCVtFblxUidJGp5\n"+
		"81hoUgZ436fZAzbdAxaFRg5Hb1IJLrnd2g9wWr4tyVUm+4QwTwfwwEpMAgDmwnInjzXgtccgNGFu\n"+
		"4XIEhRhfjPiMArQHVZ4vCIaI/YW+SZ4LZd1SsOmP/7/CX03RIeCgRp9r4OpJYoirUsP1E6HZ++m7\n"+
		"ySzGFuHO82rSnh/QjTO22/nXUMfCGaXSfWWJ+WTnCZvJCaYcHzoldnCBa1AD3FFDej+6N+ry9qpR\n"+
		"Rh44mJJaAnoLgbFiXdyAD/lOOMwt\n"))
	c.Assert(assert.Content, DeepEquals, []byte(""+
		"type: snap-revision\n"+
		"authority-id: canonical\n"+
		"snap-sha3-384: Gsqj3QgpWFq2p0517nHNNMZWgX5rG6_vaeOjT9Nyua_l36qkC_HdiDw2iEd4t1J-\n"+
		"developer-id: canonical\n"+
		"provenance: global-upload\n"+
		"snap-id: CSO04Jhav2yK0uz97cr0ipQRyqg0qQL6\n"+
		"snap-revision: 2812\n"+
		"snap-size: 58363904\n"+
		"timestamp: 2023-10-27T17:42:03.206069Z\n"+
		"sign-key-sha3-384: BWDEoaqyr25nF5SNCvEv2v7QnM9QsfCc0PBMYD_i2NGSQ32EF2d4D0hqUel3m8ul"))

	err = assert.VerifySignature(t.sl)
	c.Assert(err, IsNil)

	// Tamper with content
	assert.Content[30] = 42
	err = assert.VerifySignature(t.sl)
	c.Assert(err, ErrorMatches, ".*invalid signature: RSA verification failure")
}

func (t *snapSuite) TestSnapDeclarationAssertion(c *C) {
	encodedSnapDeclaration := `type: snap-declaration
authority-id: canonical
series: 16
snap-id: UQEdRgY5gr1dI2fwIDOgUQidMZauRqt7
publisher-id: ekRMaarzOfN1Vu3sDY0Bt1aGnM8Cd4kG
snap-name: word-salad
timestamp: 2019-02-20T20:17:43.640421Z
sign-key-sha3-384: BWDEoaqyr25nF5SNCvEv2v7QnM9QsfCc0PBMYD_i2NGSQ32EF2d4D0hqUel3m8ul

AcLBUgQAAQoABgUCXG215wAAcDcQAHjf8x7Kx5qDbywVBUUJFGGyC+EhJb45BfQvt85sMxAZGQnv
T4csx6E46N17d8f5ty/74RunA+fGsVb9B7otuWf744GDwULTANxTjNRI3LbYpjmG28/abNc12rPb
8rU557bHKS7fTO5FAyVbn5dJu7xBBQGOvhE2DFXfbIvx2Pp33zVr9d+K90RdrQO4eDvwNGE2qOcE
agI/UFW6zgTj/ayuwj6t41vE24nVEV+KcMJKkrqHBd+Pd/el7UWj5oLDVzWMKN/SUaXe7kUnRhmy
JbGQjsgNcZINYUMCaXB6bfmgWrTmkRa9qBxNr5brvj8Hxt4EwGty0Eo+tXgljmP5+4GHRMKnR+gX
M7kgIPc4XXrGRavD6lTIG+FU+wRs5vvcub3TMHTwb6Ew8poptn9sCfzKJYot+O03lZ46sRTS7erv
C2yOM1BNOnlhA1xA1/E4Hl74a6iR1T88sT4SQ4IC9QBQldtkDjaD9OmWacGz477yZOkr/8i0MM3n
VLU104b0fWlcARJ1d9RF3ZKEaBDbbZYbyP2QO51JgqpQEKWdusgqA5effZPQmLlrmPGpodt0dQU1
52Xbnz9riko6L1rNefbDUQGWFRfU8kOmCckknGH28PxwpIxlMymOTGDEvDkP4B86Gan7cpdSIt75
UXHaql1tEmG8xpLx+/SE8jFRqo/b
`
	assert, err := snap.NewAssertion([]byte(encodedSnapDeclaration))
	c.Assert(err, IsNil)

	c.Assert(assert.Type(), Equals, "snap-declaration")
	c.Assert(assert.AuthorityID(), Equals, "canonical")
	c.Assert(assert.SnapID(), Equals, "UQEdRgY5gr1dI2fwIDOgUQidMZauRqt7")
	c.Assert(assert.SnapName(), Equals, "word-salad")
	c.Assert(assert.PublisherID(), Equals, "ekRMaarzOfN1Vu3sDY0Bt1aGnM8Cd4kG")
	c.Assert(assert.Header, DeepEquals, map[string]string{
		"type":              "snap-declaration",
		"authority-id":      "canonical",
		"series":            "16",
		"snap-id":           "UQEdRgY5gr1dI2fwIDOgUQidMZauRqt7",
		"publisher-id":      "ekRMaarzOfN1Vu3sDY0Bt1aGnM8Cd4kG",
		"snap-name":         "word-salad",
		"timestamp":         "2019-02-20T20:17:43.640421Z",
		"sign-key-sha3-384": "BWDEoaqyr25nF5SNCvEv2v7QnM9QsfCc0PBMYD_i2NGSQ32EF2d4D0hqUel3m8ul",
	})
	c.Assert(assert.Body, IsNil)
	c.Assert(assert.Signature, DeepEquals, []byte(""+
		"AcLBUgQAAQoABgUCXG215wAAcDcQAHjf8x7Kx5qDbywVBUUJFGGyC+EhJb45BfQvt85sMxAZGQnv\n"+
		"T4csx6E46N17d8f5ty/74RunA+fGsVb9B7otuWf744GDwULTANxTjNRI3LbYpjmG28/abNc12rPb\n"+
		"8rU557bHKS7fTO5FAyVbn5dJu7xBBQGOvhE2DFXfbIvx2Pp33zVr9d+K90RdrQO4eDvwNGE2qOcE\n"+
		"agI/UFW6zgTj/ayuwj6t41vE24nVEV+KcMJKkrqHBd+Pd/el7UWj5oLDVzWMKN/SUaXe7kUnRhmy\n"+
		"JbGQjsgNcZINYUMCaXB6bfmgWrTmkRa9qBxNr5brvj8Hxt4EwGty0Eo+tXgljmP5+4GHRMKnR+gX\n"+
		"M7kgIPc4XXrGRavD6lTIG+FU+wRs5vvcub3TMHTwb6Ew8poptn9sCfzKJYot+O03lZ46sRTS7erv\n"+
		"C2yOM1BNOnlhA1xA1/E4Hl74a6iR1T88sT4SQ4IC9QBQldtkDjaD9OmWacGz477yZOkr/8i0MM3n\n"+
		"VLU104b0fWlcARJ1d9RF3ZKEaBDbbZYbyP2QO51JgqpQEKWdusgqA5effZPQmLlrmPGpodt0dQU1\n"+
		"52Xbnz9riko6L1rNefbDUQGWFRfU8kOmCckknGH28PxwpIxlMymOTGDEvDkP4B86Gan7cpdSIt75\n"+
		"UXHaql1tEmG8xpLx+/SE8jFRqo/b\n"))

	err = assert.VerifySignature(t.sl)
	c.Assert(err, IsNil)
}

func (t *snapSuite) TestAssertionWithMultilineEntries(c *C) {
	encodedAssertion := `type: snap-declaration
authority-id: canonical
revision: 2
series: 16
snap-id: Md1HBASHzP4i0bniScAjXGnOII9cEK6e
aliases:
  -
    name: gofmt
    target: gofmt
auto-aliases:
  - gofmt
publisher-id: canonical
snap-name: go
timestamp: 2023-03-21T15:47:07.267177Z
sign-key-sha3-384: BWDEoaqyr25nF5SNCvEv2v7QnM9QsfCc0PBMYD_i2NGSQ32EF2d4D0hqUel3m8ul

AcLBUgQAAQoABgUCZBnRewAA0PwQAIvALUmv5/M8EMX7z8lZ5ZAsthrxKtgVsdvidvSoz0S2YUSi
4TSRPzrCGnnO6AeBexscmBDjaoryqrAZU2wxMoC2Sc5AqseaGoIGQLwD2FeKhbXG3oNi71eDmoRZ
p8bqteh8L+wlbPMBz6dpk2ZtfqtZhaJt6xr5OJ3xLp2a1x22TakCvbWO2KmSW40Qs6eozlpYPquO
gbScrv5eVirelbLco0hE6SbpqQDtsqnTbigXP5vd3zHTpBK8usEGL/fURU2MwcUDSQUj8rV+N1AW
6VrcDyafo5wlQAoG2+YGD+p2DbMFudFXp6fIYYk9Dm3G6aDUZi9W+ZW+HynlWoebG8ngWn1S+iyj
o/YFPicXztw1q32bgjxolgh1l59GmBj8eH+FIDJf1UUir1tqGawLA/CYfCkHTIONnTWRbOx2Byqr
D49zR47qe8mPvyWeSizOb61vWEhwfDtYgPReuhrwa5VB0xjDEuXr5taukJf252/Bz45BQXy2RF4C
mLJPpODoN33+OCzyvt62VSf8cJe1dHhQQslP6rIES8mbCxO987xqMpWw0DJvMJ5RnWyXbW3dyt3e
5aZ4ondJYabrk41d4SdgSah8LF0MmlTlmXgV87cCZArmWjD0NP2tLjfTpDqSfg4Oobr4qU9GDUpQ
wGVm0wR2xbVTB4tKHlD6KJXc0unK
`
	assert, err := snap.NewAssertion([]byte(encodedAssertion))
	c.Assert(err, IsNil)

	c.Assert(assert.Type(), Equals, "snap-declaration")
	c.Assert(assert.AuthorityID(), Equals, "canonical")
	c.Assert(assert.SnapID(), Equals, "Md1HBASHzP4i0bniScAjXGnOII9cEK6e")
	c.Assert(assert.SnapName(), Equals, "go")
	c.Assert(assert.PublisherID(), Equals, "canonical")
	c.Assert(assert.Header, DeepEquals, map[string]string{
		"type":              "snap-declaration",
		"authority-id":      "canonical",
		"series":            "16",
		"revision":          "2",
		"snap-id":           "Md1HBASHzP4i0bniScAjXGnOII9cEK6e",
		"publisher-id":      "canonical",
		"snap-name":         "go",
		"timestamp":         "2023-03-21T15:47:07.267177Z",
		"sign-key-sha3-384": "BWDEoaqyr25nF5SNCvEv2v7QnM9QsfCc0PBMYD_i2NGSQ32EF2d4D0hqUel3m8ul",
		"aliases":           "", // multi-line entries not supported yet
		"auto-aliases":      "", // multi-line entries not supported yet
	})
	c.Assert(assert.Body, IsNil)
	c.Assert(assert.Signature, DeepEquals, []byte(""+
		"AcLBUgQAAQoABgUCZBnRewAA0PwQAIvALUmv5/M8EMX7z8lZ5ZAsthrxKtgVsdvidvSoz0S2YUSi\n"+
		"4TSRPzrCGnnO6AeBexscmBDjaoryqrAZU2wxMoC2Sc5AqseaGoIGQLwD2FeKhbXG3oNi71eDmoRZ\n"+
		"p8bqteh8L+wlbPMBz6dpk2ZtfqtZhaJt6xr5OJ3xLp2a1x22TakCvbWO2KmSW40Qs6eozlpYPquO\n"+
		"gbScrv5eVirelbLco0hE6SbpqQDtsqnTbigXP5vd3zHTpBK8usEGL/fURU2MwcUDSQUj8rV+N1AW\n"+
		"6VrcDyafo5wlQAoG2+YGD+p2DbMFudFXp6fIYYk9Dm3G6aDUZi9W+ZW+HynlWoebG8ngWn1S+iyj\n"+
		"o/YFPicXztw1q32bgjxolgh1l59GmBj8eH+FIDJf1UUir1tqGawLA/CYfCkHTIONnTWRbOx2Byqr\n"+
		"D49zR47qe8mPvyWeSizOb61vWEhwfDtYgPReuhrwa5VB0xjDEuXr5taukJf252/Bz45BQXy2RF4C\n"+
		"mLJPpODoN33+OCzyvt62VSf8cJe1dHhQQslP6rIES8mbCxO987xqMpWw0DJvMJ5RnWyXbW3dyt3e\n"+
		"5aZ4ondJYabrk41d4SdgSah8LF0MmlTlmXgV87cCZArmWjD0NP2tLjfTpDqSfg4Oobr4qU9GDUpQ\n"+
		"wGVm0wR2xbVTB4tKHlD6KJXc0unK\n"))

	err = assert.VerifySignature(t.sl)
	c.Assert(err, IsNil)
}
