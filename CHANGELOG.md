# Changelog

## 0.1.0 (2026-10-07)


### Features

* **clef:** add Cloudflare clef as a second System One backend ([8238a3a](https://github.com/rshade/go-decide/commit/8238a3aeaf9472ed11f81de51e7fdd290e70640c))
* **clef:** add Cloudflare clef as a second System One backend ([a2b8bea](https://github.com/rshade/go-decide/commit/a2b8bea86debe7cc88a95bbe2a7b593a4a4f4d72)), closes [#18](https://github.com/rshade/go-decide/issues/18)
* **cli:** add ask and score commands with versioned JSON output ([418f392](https://github.com/rshade/go-decide/commit/418f3920eade67b5a5737e2dd6dbe8a440fd079a)), closes [#4](https://github.com/rshade/go-decide/issues/4)
* **cli:** add eval command for confidence separation and calibration ([64ac643](https://github.com/rshade/go-decide/commit/64ac643d4b6b77e7e31fc394890b29059364983a)), closes [#5](https://github.com/rshade/go-decide/issues/5)
* **client:** add jevclient adapter and decision result types ([2a2600d](https://github.com/rshade/go-decide/commit/2a2600dd422252b4c00710fd944cb11c10c851cd))
* **client:** add jevclient adapter and decision result types ([6f4f07e](https://github.com/rshade/go-decide/commit/6f4f07e818ba1a77473f6d25917324b1203579d0)), closes [#1](https://github.com/rshade/go-decide/issues/1) [#2](https://github.com/rshade/go-decide/issues/2)
* **decision:** validate the question before any API call ([c1c6c5e](https://github.com/rshade/go-decide/commit/c1c6c5e1efa0240cef08bff473e9e026ebbab4c9))
* **decision:** validate the question before any API call ([29fa25b](https://github.com/rshade/go-decide/commit/29fa25b24feeffb9e73ee55ae6c5bc4131b5ec1d)), closes [#3](https://github.com/rshade/go-decide/issues/3)
* make go-decide safe to publish at v0.1.0 ([195291b](https://github.com/rshade/go-decide/commit/195291ba3be7932d1fe4276b8bf00a4817ba6c36)), closes [#12](https://github.com/rshade/go-decide/issues/12) [#13](https://github.com/rshade/go-decide/issues/13) [#16](https://github.com/rshade/go-decide/issues/16) [#17](https://github.com/rshade/go-decide/issues/17) [#19](https://github.com/rshade/go-decide/issues/19)


### Bug Fixes

* **ci:** split over-long OpenSpec requirements and refresh README limits ([#31](https://github.com/rshade/go-decide/issues/31)) ([b075b69](https://github.com/rshade/go-decide/commit/b075b69e5c2697031cf6e8373d2ebd32c37ddd67)), closes [#20](https://github.com/rshade/go-decide/issues/20)
* **clef:** name clef in config errors, archive add-clef-backend, harden commitlint ([#32](https://github.com/rshade/go-decide/issues/32)) ([e41cb04](https://github.com/rshade/go-decide/commit/e41cb04fed0a0321355c8b9b53042178d4951305)), closes [#20](https://github.com/rshade/go-decide/issues/20)
* **cli:** reject empty instructions on the clef backend ([3791b29](https://github.com/rshade/go-decide/commit/3791b292c8a4d31bdf79618d703150e7e21bda67))
* drop the unfinished clef client import ([d9ff612](https://github.com/rshade/go-decide/commit/d9ff6126dade2b973c879d6e21f44a8dff8d81db))
* print the same JSON from a repeated dry run ([9eec22b](https://github.com/rshade/go-decide/commit/9eec22b4ebf3241f7547bf2949febd97cf180393))
* **release:** let the release PR move the manifest off 0.0.0 ([#30](https://github.com/rshade/go-decide/issues/30)) ([1ff3148](https://github.com/rshade/go-decide/commit/1ff3148c1a14fdc0410732d08abaefaf75472dac)), closes [#20](https://github.com/rshade/go-decide/issues/20)
* report integer schema_version 3 from __schema ([ec831cc](https://github.com/rshade/go-decide/commit/ec831ccedfd0968381fac2c7ebfa239e823a2970))


### Documentation

* add CONTEXT.md and ROADMAP.md ([40b53b8](https://github.com/rshade/go-decide/commit/40b53b809f2bcdebe85b5c2c9ff498edf3114019))
* add Jev probes, spikes and client survey ([ea05df9](https://github.com/rshade/go-decide/commit/ea05df9d22a767c50be86f52d05a66a33929d2e0))
* add project CLAUDE.md ([a424bec](https://github.com/rshade/go-decide/commit/a424bec0462592521cd6ba54abf39e426365b1a1))
* add the v0.1.0 task breakdown ([2af1ead](https://github.com/rshade/go-decide/commit/2af1eadaaaf77b712b77f610ba2b6032760bebf3))
* close spike 10 and move it out of the roadmap ([5dc61c8](https://github.com/rshade/go-decide/commit/5dc61c88ac0d2c36cc59b9ca04e9bcc21aba8a2f))
* rewrite the README for people using the CLI ([6e1d813](https://github.com/rshade/go-decide/commit/6e1d813e6f86cad14dd1374bcccaf083a16cfa9d))
* track release progress and the MCP and decide skill tasks ([e1ed22e](https://github.com/rshade/go-decide/commit/e1ed22ec76b95d092bedcfa26e5afd266420a60c))
