*********
Changelog
*********

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
