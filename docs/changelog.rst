*********
Changelog
*********

0.15.2 (2026-03-20)
-------------------

- fix(deps): update elazarl/goproxy to 1.8.2 (#586)

0.15.1 (2026-03-19)
-------------------

- fix(inspectors/apt): recognize small translation files (#579)
- chore: update bld and store api urls (#580)
- chore: change redacted urls to avoid % encoding (#570)

0.15.0 (2026-02-12)
-------------------

- feat: add upstream_proxy.bypass configuration option (#567)
- feat: add keystone-v3 secrets (#565)
- fix(inspectors/lxd): allow dot in image date (#566)

0.14.0 (2026-01-26)
-------------------

- feat: specify and use upstream proxy (#562)
- feat(i/lxd): support additional instance type file formats (#563)

0.13.0 (2026-01-13)
-------------------

- feat: reopen log file if SIGUSR1 received (#553)
- fix: prevent race when copying http response (#541)
- fix(inspectors/snap): don't reject non-snap squashfs files (#544)
- fix(inspectors/apt): allow translation file request by name (#548)
- fix(inspectors/lxd): add 26.04 to the list of supported versions (#557)

0.12.0 (2025-12-01)
-------------------

- feat: add 'macaroon' secret type (#532)
- fix: add artifact type for snap store icon (#530)
- fix(i/maven): parse jar file regardless of url (#535)

0.11.2 (2025-11-25)
-------------------

- fix: increase inspection timeout to 5 minutes (#536)

0.11.1 (2025-11-18)
-------------------

- fix: less restrictive assertion url inspection (#524)
- fix: add appmedia entry to default configuration (#523)
- fix(inspectors/snap): fix serial assertion endpoint (#522)
- test(spread): add an improved snap inspector test (#521)

0.11.0 (2025-11-17)
-------------------

- feat: add inspector for lxd instance types (#504)
- feat: add inspector for device authentication request-id (#519)
- feat: add inspector for snap package names (#517)
- feat: add support for serial assertion inspection (#516)
- feat: add snap sections inspector (#508)
- feat: add inspector for snapd session authentication (#507)
- feat: add inspector for snapd authentication nonce (#503)
- feat: add inspector for store media (#482)
- feat: add format-specific id to metadata (#492)
- feat: add optional apt-suite to artifact metadata (#476)
- feat: support per-session inspector conf (#480)
- feat(secrets): support for 'basic' secrets (#483)
- feat(config): override bundled config with custom one (#497)
- fix: check if session exists before completing inspection (#489)
- fix: exclude slash from glob star matching (#484)

0.10.1 (2025-10-17)
-------------------

- feat: add inspector for the store transforms api (#470)
- fix(inspectors/git): allow packets containing only sideband (#467)
- fix: false positive in store info inspection (#477)

0.10.0 (2025-10-02)
-------------------

- feat(inspectors): add chisel-releases inspector (#445)
- feat(inspectors): add inspector for the store resolve API (#450)
- feat: introduce apt repository aliases (#455)
- fix(inspectors/lxd): detect releases image request (#462)
- fix(inspectors/gomod): set metadata if artifact approved (#465)
- fix(deps): update module github.com/gabriel-vasile/mimetype to v1.4.10 (#449)
- fix(deps): bump github.com/ulikunitz/xz from 0.5.12 to 0.5.15 (#457)
- fix(deps): update module golang.org/x/net to v0.43.0 (#436)
- refactor: consolidate duplicated code chunks (#459)

0.9.0 (2025-08-14)
------------------

- feat(i/lxd): simple streams index inspector for LXD (#423)
- feat(i/lxd): simple streams download inspector for lxd (#427)
- feat(i/lxd): add lxd rootfs inspector (#429)
- fix(i/apt): prevent false positives in deb inspection (#422)
- fix(i/apt): keep existing packages metadata in inspector state (#428)
- fix(i/bldbin): inspect bin requests in store inspector (#439)
- fix(deps): update module golang.org/x/crypto to v0.40.0 (#405)
- fix(deps): update module github.com/gabriel-vasile/mimetype to v1.4.9 (#380)
- fix(deps): update module golang.org/x/net to v0.42.0 (#406)

0.8.1 (2025-07-11)
------------------

- fix: address download timeout issues (#412)
- fix snap declaration assertion parsing (#414)

0.8.0 (2025-07-02)
------------------

- feat: add charmcraft inspector (#397)
- feat: add store api and bld bin package inspectors (#398, #400)
- feat: add apt commands file inspector (#395, #402)
- feat: add session-aware logging support (#371)
- fix: prevent races in cached inspection results (#409)
- fix(apt): use separate esm repository configuration (#410)
- fix(deps): update module github.com/protonmail/go-crypto to v1.3.0 (#390)
- fix(deps): update golang.org/x/net to 0.38.0 (#387)

0.7.0 (2025-04-08)
------------------

- feat!(config): default to bundled configuration (#370)
- feat!: mark requests for external origins as unknown (#368)
- fix(inspectors/apt): use correct esm archive url (#365)
- fix(inspectors/apt): make InRelease description field optional (#334)
- fix!(cli): return proper error codes on start error (#375)
- fix(deps): update module github.com/klauspost/compress to v1.18.0 (#340)
- fix(deps): update go-crypto to v1.1.5 (#348)
- fix(deps): update module github.com/elazarl/goproxy to v1.7.2 (#349)
- fix(deps): update module github.com/gabriel-vasile/mimetype to v1.4.8 (#297)

0.6.1 (2025-02-21)
------------------

- feat(inspectors/apt): allow packages file download by name (#338)
- fix(inspectors/snap): use version as assigned by vendor (#341)
- fix(inspectors/deb): handle more compression methods (#339)
- fix(inspectors/deb): remove incorrect metadata mapping (#328)
- fix(deps): update module golang.org/x/net to v0.35.0 (#321)

0.6.0 (2025-01-13)
------------------

- feat(proxy): validate session id from request header (#313)
- fix: address tiobe security warnings (#307, #317)
- fix(inspectors/deb): check scan return value (#312)
- fix(inspectors/apt): rejection fix and additional tests (#308)
- fix(deps): update module golang.org/x/net to v0.33.0 [security] (#318)
- refactor(service): handle messages in separate functions (#315)
- docs: local testing howto (#263)
- chore!: change artefact spelling to artifact (#306)

0.5.0 (2024-11-27)
------------------

- feat: allow logging to a file (#294)
- refactor: only unpack git repos once (#290)
- chore(deps): update dependency canonical-sphinx to v0.2.0 (#235)
- fix(snap): set the service to endure mode (#273)

0.4.3 (2024-10-29)
------------------

- fix(inspectors/git): handle absence of proto and command (#283)

0.4.2 (2024-10-23)
------------------

- allow reading very short packages files (#279)

0.4.1 (2024-10-22)
------------------

- feat: add fetch-service version to session metadata (#272)
- fix(service): apply same rules if artefact already downloaded (#275)
- fix(deps): update module github.com/gabriel-vasile/mimetype to v1.4.6 (#270)
- fix: add build-aux location for snapcraft project (#274)
- tests: add initial spread testing (#269)

0.4.0 (2024-10-14)
------------------

- feat(inspectors): add snapcraft inspector (#264)
- feat(inspectors): Rockcraft inspector (#262)
- fix(proxy): normalize request urls (#266)
- fix(inspectors/pip): adjust wheel url pattern (#265)
- fix(deps): update module github.com/klauspost/compress to v1.17.11 (#267)

0.3.0 (2024-10-10)
------------------

- apt, snap, git, craft inspectors configurable at runtime
- refactor: remove unnecessary local loading of apt public key (#255)
- fix(service): reject unknown request in strict mode (#253)
- fix(i/apt): check for built-using and allow ubuntu-ports (#251)
- fix(deps): update module golang.org/x/net to v0.30.0 (#205)

0.2.3 (2024-10-03)
------------------

- fix: don't fail parsing if line has no ':'
- refactor: separate function to parse deb822 packages
- fix: fail if FETCH_APT_RELEASE_PUBLIC_KEY is not set

0.2.2 (2024-09-30)
------------------

- fix(i/pip): recognize pypi index version 1.2 (#234)

0.2.1 (2024-09-25)
------------------

- fix: remove inspecting text/html in sourcecraft inspector (#229)

0.2.0 (2024-09-25)
------------------

- service: implement idle auto-shutdown (#183)
- feat(service): add configuration socket server (#190)
- feat(config): add service configuration client (#191)
- feat: add Maven POM inspector (#192)
- feat(config): add update-config command (#199)
- feat(config): add update-cert command (#206)
- feat: add sourcecraft inspector (#217)

0.1.4 (2024-07-30)
------------------

- fix(snap): update config dir in post-refresh hook (#171)
- fix(control): don't escape HTML  in session report (#167)
- fix(inspectors/git): make artefact verification less strict (#166)

0.1.3 (2024-07-25)
------------------

- fix(metadata): don't overwrite existing artefact type (#162)
- metadata,inspectors: fix inspection result and add tests (#158)
- inspectors/snap: add additional inspector tests (#150)
- control: fix error handling and add tests (#151)
- session: fill policy field in metadata (#142)
- feat: add cargo inspectors (#144)
- inspectors/snap: fix snap refresh inspector (#145)
- inspectors: reorganize artefact file interface (#143)
- inspectors/git: deduplicate wanted refs (#141)
- inspectors: don't mmap files for mime type detection (#139)
- many: limit inspector access to artefact metadata (#140)
- inspectors/pip: add metadata file inspector (#137)
- inspectors/pip: add sdist inspector (#113)
- service: move configuration files to subdirectory (#122)
- control: remove status endpoint authentication (#136)
- fix: further adjust command line quoting in snap wrapper

0.1.2 (2024-07-10)
------------------

- basic functionality
