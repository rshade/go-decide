// Subjects of pushed commits must be conventional commits. Body lines are
// not linted: commitlint v21 reads a body line that starts with "word:" as
// a footer, and this repo wraps bodies at 72 columns by hand.
module.exports = {
  rules: {
    "type-empty": [2, "never"],
    "subject-empty": [2, "never"],
    "type-enum": [
      2,
      "always",
      [
        "feat",
        "fix",
        "docs",
        "style",
        "refactor",
        "perf",
        "test",
        "build",
        "ci",
        "chore",
        "revert",
      ],
    ],
  },
};
