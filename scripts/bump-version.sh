#!/bin/bash
set -e

# Bump version script for dap-mcp
# Usage: ./scripts/bump-version.sh [patch|minor|major]

BUMP_TYPE="${1:-patch}"
ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"

# Get current version from Makefile
CURRENT_VERSION=$(grep "^VERSION?=" "$ROOT_DIR/Makefile" | cut -d= -f2)

if [ -z "$CURRENT_VERSION" ]; then
    echo "Error: Could not determine current version from Makefile"
    exit 1
fi

# Parse version components
IFS='.' read -r MAJOR MINOR PATCH <<< "$CURRENT_VERSION"

# Calculate new version
case "$BUMP_TYPE" in
    patch)
        PATCH=$((PATCH + 1))
        ;;
    minor)
        MINOR=$((MINOR + 1))
        PATCH=0
        ;;
    major)
        MAJOR=$((MAJOR + 1))
        MINOR=0
        PATCH=0
        ;;
    *)
        echo "Usage: $0 [patch|minor|major]"
        exit 1
        ;;
esac

NEW_VERSION="$MAJOR.$MINOR.$PATCH"

echo "Bumping version: $CURRENT_VERSION -> $NEW_VERSION"

# Files to update
FILES=(
    "Makefile:VERSION?=$CURRENT_VERSION:VERSION?=$NEW_VERSION"
    "cmd/dap-mcp/main.go:version = \"$CURRENT_VERSION\":version = \"$NEW_VERSION\""
    "internal/mcp/server.go:\"$CURRENT_VERSION\",:\"$NEW_VERSION\","
    "packaging/obs/dap-mcp.spec:Version:        $CURRENT_VERSION:Version:        $NEW_VERSION"
    "packaging/alpine/APKBUILD:pkgver=$CURRENT_VERSION:pkgver=$NEW_VERSION"
    "packaging/arch/PKGBUILD:pkgver=$CURRENT_VERSION:pkgver=$NEW_VERSION"
    "packaging/arch/dap-mcp-bin-PKGBUILD:pkgver=$CURRENT_VERSION:pkgver=$NEW_VERSION"
)

# Update each file
for entry in "${FILES[@]}"; do
    IFS=':' read -r file old new <<< "$entry"
    filepath="$ROOT_DIR/$file"
    if [ -f "$filepath" ]; then
        if grep -q "$old" "$filepath"; then
            sed -i '' "s|$old|$new|g" "$filepath"
            echo "  Updated $file"
        else
            echo "  Warning: Pattern not found in $file"
        fi
    else
        echo "  Warning: $file not found"
    fi
done

# Update README.md (all occurrences)
README="$ROOT_DIR/README.md"
if [ -f "$README" ]; then
    sed -i '' "s/$CURRENT_VERSION/$NEW_VERSION/g" "$README"
    echo "  Updated README.md"
fi

echo ""
echo "Version bumped to $NEW_VERSION"
echo ""
echo "Next steps:"
echo "  1. Review changes: git diff"
echo "  2. Commit: git add -A && git commit -m \"Bump version to $NEW_VERSION\""
echo "  3. Tag: git tag v$NEW_VERSION"
echo "  4. Push: git push origin main && git push origin v$NEW_VERSION"
