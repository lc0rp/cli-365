const plugins = [
  [
    "@semantic-release/commit-analyzer",
    {
      preset: "conventionalcommits"
    }
  ],
  [
    "@semantic-release/release-notes-generator",
    {
      preset: "conventionalcommits"
    }
  ],
  [
    "@semantic-release/changelog",
    {
      changelogFile: "CHANGELOG.md"
    }
  ],
  [
    "@semantic-release/exec",
    {
      verifyReleaseCmd: "scripts/verify-release.sh ${nextRelease.version}",
      prepareCmd: "scripts/set-version.sh ${nextRelease.version}"
    }
  ],
  [
    "@semantic-release/git",
    {
      assets: ["CHANGELOG.md", "VERSION", "cmd/cli-365/version.go"],
      message: "chore(release): ${nextRelease.version} [skip ci]\n\n${nextRelease.notes}"
    }
  ]
];

if (process.env.GITHUB_ACTIONS === "true" || process.env.GITHUB_TOKEN) {
  plugins.push("@semantic-release/github");
}

module.exports = {
  branches: ["main"],
  tagFormat: "v${version}",
  plugins
};
