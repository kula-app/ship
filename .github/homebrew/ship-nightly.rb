class ShipNightly < Formula
  desc "CLI for Shipable app deployment workflows (nightly)"
  homepage "https://github.com/kula-app/ship"
  version "{{VERSION}}"

  # Rolling build published from the latest commit on `main`. The `latest`
  # GitHub release is overwritten on every push, so the download URL never
  # changes; `version` is derived from the commit timestamp so that
  # `brew upgrade` always treats a newer build as an upgrade.
  on_macos do
    on_arm do
      url "https://github.com/kula-app/ship/releases/download/latest/ship-darwin-arm64"
      sha256 "{{SHA_DARWIN_ARM64}}"
    end
    on_intel do
      url "https://github.com/kula-app/ship/releases/download/latest/ship-darwin-amd64"
      sha256 "{{SHA_DARWIN_AMD64}}"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/kula-app/ship/releases/download/latest/ship-linux-arm64"
      sha256 "{{SHA_LINUX_ARM64}}"
    end
    on_intel do
      url "https://github.com/kula-app/ship/releases/download/latest/ship-linux-amd64"
      sha256 "{{SHA_LINUX_AMD64}}"
    end
  end

  # Installs the same `ship` binary as the stable formula, so the two channels
  # cannot be linked at the same time. Switch channels by uninstalling one and
  # installing the other.
  conflicts_with "ship", because: "both install a ship binary"

  def install
    binary = Dir["ship-*"].first
    bin.install binary => "ship"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/ship --version")
  end
end
