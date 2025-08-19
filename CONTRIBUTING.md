# CONTRIBUTING

Thank you for considering contributing in the BSV Blockchain ecosystem! This document outlines the processes and practices we expect contributors to adhere to.

## Table of Contents
- [General Guidelines](#general-guidelines)
- [Code of Conduct](#code-of-conduct)
  - [Posting Issues and Comments](#posting-issues-and-comments)
  - [Coding and PRs](#coding-and-prs)
- [Getting Started](#getting-started)
- [Pull Request Process](#pull-request-process)
- [Coding Conventions](#coding-conventions)
- [Documentation and Testing](#documentation-and-testing)
- [Contact \& Support](#contact--support)

## General Guidelines

- **Issues First**: If you're planning to add a new feature or change existing behavior, please open an issue first. This allows us to avoid multiple people working on similar features and provides a place for discussion.

- **Stay Updated**: Always pull the latest changes from the main branch before creating a new branch or starting on new code.

- **Simplicity Over Complexity**: Your solution should be as simple as possible, given the requirements.

## Code of Conduct

### Posting Issues and Comments

- **Be Respectful**: Everyone is here to help and grow. Avoid any language that might be considered rude or offensive.

- **Be Clear and Concise**: Always be clear about what you're suggesting or reporting. If an issue is related to a particular piece of code or a specific error message, include that in your comment.

- **Stay On Topic**: Keep the conversation relevant to the issue at hand. If you have a new idea or unrelated question, please open a new issue.

### Coding and PRs

- **Stay Professional**: Avoid including "fun" code, comments, or irrelevant file changes in your commits and pull requests.

### Commit Messages

- **Use Conventional Commits**: To maintain a clean, organized, and automated commit history, we follow the Conventional Commits format.
This format allows for clear categorization of commits, enabling automated changelogs and improved collaboration.


Each commit message should follow this structure:

```
<type>(<scope>): <description>
```


Where:

`<type>`: Specifies the purpose of the change. Options implemented when sorting changelog:
```
deps     -> dependency update
feat     -> new feature
chore    -> updating grunt tasks, no production code change
test     -> changes associated with the tests
sec      -> security update
fix      -> bug fixes
refactor -> refactor commits
docs     -> documentation
build/ci -> build process update
```

`<scope>`: **(Optional)** Task or issue identifier, Example: **ARCO-001**\
`<description>`: A brief description of the change.

Example of a commit:

> `feat(ARCO-001): add new feature to the project`

## Getting Started

1. **Fork the Repository**: Click on the "Fork" button at the top-right corner of this repository.

2. **Clone the Forked Repository**: `git clone https://github.com/YOUR_USERNAME/PROJECT.git`

3. **Navigate to the Directory**: `cd PROJECT`

4. **Install Dependencies**: Always run `go mod download` or `task deps` after pulling to ensure tooling is up to date.

## Pull Request Process

1. **Create a Branch**: For every new feature or bugfix, create a new branch.

2. **Commit Your Changes**: Make your changes and commit them. Commit messages should be clear and concise to explain what was done.

3. **Run Tests**: Ensure all tests pass.

4. **Documentation**: All code must be fully annotated with comments. Update the documentation by running `task docs` before creating a pull request.

5. **Push to Your Fork**: `git push origin your-new-branch`.

6. **Open a Pull Request**: Go to your fork on GitHub and click "New Pull Request". Fill out the PR template, explaining your changes.

7. **Code Review**: At least two maintainers must review and approve the PR before it's merged. Address any feedback or changes requested.

8. **Merge**: Once approved, the PR will be merged into the main branch.

## Coding Conventions

- **Code Style**: We use `Effective Go` guideline for our coding style. Run `task lint` to ensure your code adheres to this style.

- **Testing**: Always include tests for new code or changes. We aim for industry-standard levels of test coverage.

- **Documentation**: All exported functions, structures and modules should be documented. Use comments to describe the purpose, parameters, and return values.

## Documentation and Testing

- **Documentation**: Update the documentation whenever you add or modify the code. Run `task docs` to generate the latest docs.

- **Testing**: Write comprehensive tests, ensuring edge cases are covered. All PRs should maintain or improve the current test coverage.

## Contact & Support

If you have any questions or need assistance with your contributions, feel free to reach out. Remember, we're here to help each other grow and improve the ecosystem.

Thank you for being a part of this journey. Your contributions help shape the future of the BSV Blockchain!
