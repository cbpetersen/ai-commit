# Maintainer: Christoffer Petersen <tools@cb-p.dk>
pkgname=ai-commit
# pkgver=0.1.0
pkgrel=1
pkgdesc="CLI tool to commit to git with AI generated messages"
arch=('x86_64')
url="https://github.com/cbpetersen/ai-commit"
license=('MIT')
depends=('go')
makedepends=('git' 'go')
source=("git+$url.git#tag=v$pkgver")
build() {
    cd "$srcdir/$pkgname"
    go build -o "$pkgname"
}

pkgver() {
    cd "$pkgname"
    (
        set -o pipefail
        git describe --long --abbrev=7 2>/dev/null | sed 's/\([^-]*-g\)/r\1/;s/-/./g' ||
            printf "r%s.%s" "$(git rev-list --count HEAD)" "$(git rev-parse --short=7 HEAD)"
    )
}

package() {
    install -Dm755 "$srcdir/$pkgname/$pkgname" "$pkgdir/usr/bin/$pkgname"
    install -Dm644 "$srcdir/$pkgname/LICENSE" "$pkgdir/usr/share/licenses/$pkgname/LICENSE"
}

sha256sums=('SKIP')
