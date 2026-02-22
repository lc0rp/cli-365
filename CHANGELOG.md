## [1.0.1](https://github.com/lc0rp/cli-365/compare/v1.0.0...v1.0.1) (2026-02-22)

### Bug Fixes

* **calendar:** fall back from stale daemon flag parsing ([ddee643](https://github.com/lc0rp/cli-365/commit/ddee64304926c568a127c7cc1ee7542eb0dda8c4))

## 1.0.0 (2026-02-22)

### Features

* add calendar CRUDL support ([9fc9123](https://github.com/lc0rp/cli-365/commit/9fc9123a8d4591ec96fa146517ffa79501753777))
* add security features (readonly, allowlist, keyring) ([1db7258](https://github.com/lc0rp/cli-365/commit/1db7258e519301d4f8ccaf8e410d7e0e37a81dac))
* **browser:** add no_sandbox option ([6b46138](https://github.com/lc0rp/cli-365/commit/6b46138dbc5b96425d03640931ebf9684590d873))
* **calendar:** add directory calendar flows and list selector ([601d9a7](https://github.com/lc0rp/cli-365/commit/601d9a758d1c86d87c85c2e21b093f2bcd4ae2bd))
* **daemon:** add auth recovery and retry safeguards ([8459f18](https://github.com/lc0rp/cli-365/commit/8459f189a89256c08fa85fa74de7031f055f999a))
* **daemon:** add best-effort owa tab recovery ([6946af2](https://github.com/lc0rp/cli-365/commit/6946af20c785175943f2605cac14d88c6d5e977d))
* **daemon:** add in-process daemon dispatch and queue runtime ([29f86e5](https://github.com/lc0rp/cli-365/commit/29f86e5b3af9c2159319e4f11cd9a18a0d963fc8))
* **daemon:** add periodic session maintenance loop ([dfd8471](https://github.com/lc0rp/cli-365/commit/dfd84718a9c2a049ace95680da206ffe73bfebf1))
* **daemon:** add primary tab cleanup baseline ([8ee6dc8](https://github.com/lc0rp/cli-365/commit/8ee6dc801b2a8ccc2d11a3902ad27482385002f7))
* **daemon:** add redacted structured logs ([3eb4086](https://github.com/lc0rp/cli-365/commit/3eb4086f8ba23fda13ee4231752f2374ec6e762e))
* **daemon:** add session preflight and token refresh ([aea3389](https://github.com/lc0rp/cli-365/commit/aea33890f461997eef309c6734200c3fd999386f))
* **daemon:** cleanup managed browser state on stop ([6d2ba8f](https://github.com/lc0rp/cli-365/commit/6d2ba8f535151f181a56ee9173f3511c2b87cba1))
* **daemon:** harden startup auth flow and live status probes ([e47803a](https://github.com/lc0rp/cli-365/commit/e47803a35d912bb8d047d29008c6acf05bf581d8))
* **daemon:** preflight notifier command availability ([afc0d1b](https://github.com/lc0rp/cli-365/commit/afc0d1bae2120b517079d696b451dc1c77e14180))
* **daemon:** recover unavailable session probe via browser start ([01cdc8b](https://github.com/lc0rp/cli-365/commit/01cdc8ba816113564a0f929d90fa26370a0947b2))
* **daemon:** wire browser start/stop into tab manager ([102bbd4](https://github.com/lc0rp/cli-365/commit/102bbd4f4ec7b6030fab933e17bf962d5c7094a2))
* **debug:** add discover command with template + network logging ([27f7266](https://github.com/lc0rp/cli-365/commit/27f7266dcde33e719f8841be265459bc9e4eff88))
* improve token discovery and add outlook.cloud.microsoft support ([d5e968b](https://github.com/lc0rp/cli-365/commit/d5e968b7a8f392c49a720cca41520dac17faff54))
* **mail:** add view command + comprehensive test coverage ([428428b](https://github.com/lc0rp/cli-365/commit/428428b0ef7a6f545a92a552c9b5bbad4e929e96))
* **mail:** improve read flows and docs ([dea14cf](https://github.com/lc0rp/cli-365/commit/dea14cf5192099fa9144fe8431f1fd61ace0a771))
* **mail:** improve reply/send and add index cache ([36a80a4](https://github.com/lc0rp/cli-365/commit/36a80a402c612ba70a422af2056799913113b60e))
* **owa:** add app-aware endpoints for calendar ([94c89f8](https://github.com/lc0rp/cli-365/commit/94c89f80b61f9cca4e3bdabb8f1b82ef7a99d553))
* **owa:** add OWA discovery layer and mail operations ([86ff9d2](https://github.com/lc0rp/cli-365/commit/86ff9d2f3a7e4409802687b876ee35856e24d83a))
* **release:** add semver versioning and automated release workflow ([8d76e58](https://github.com/lc0rp/cli-365/commit/8d76e5837d24b53403ae18f9f76bcb41cd118f43))
* scaffold outlook-browser-cli ([b278764](https://github.com/lc0rp/cli-365/commit/b278764ce17fc8d5c051e8a2d7d33de777095c72))

### Bug Fixes

* **calendar:** retry list with FindItemJsonRequest ([ad1593c](https://github.com/lc0rp/cli-365/commit/ad1593c183e5041d33b37b93bac8a14d327b1d29))
* **ci:** require node 22 and stabilize daemon macOS smoke ([84b002b](https://github.com/lc0rp/cli-365/commit/84b002b5c577d6da681c1cf5a2f05250bc926588))
* **daemon:** keep persistent cdp connection for tab manager ([f69ac9c](https://github.com/lc0rp/cli-365/commit/f69ac9ca55e0ef9e1274c278b9f1897b34141ba7))
* handle compact limit flags ([50f93f0](https://github.com/lc0rp/cli-365/commit/50f93f0779efef96d87b4e4de829e6354de5e61d))
* **owa:** harden calendar actions and auth fallbacks ([a2e66f8](https://github.com/lc0rp/cli-365/commit/a2e66f8bb5eea2b1aba633bd6966b593a9f9948d))

# Changelog

All notable changes to this project are documented in this file.

The format follows Keep a Changelog and semantic versioning (`vMAJOR.MINOR.PATCH` tags).

## Unreleased

- Initial changelog scaffold.
