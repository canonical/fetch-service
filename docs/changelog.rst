*********
Changelog
*********

0.7.0 (2025-04-07)
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
