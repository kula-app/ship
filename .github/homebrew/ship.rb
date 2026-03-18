class Ship < Formula
  desc "CLI for Shipable app deployment workflows"
  homepage "https://github.com/kula-app/ship"
  version "{{VERSION}}"

  on_macos do
    on_arm do
      url "https://github.com/kula-app/ship/releases/download/v{{VERSION}}/ship-darwin-arm64"
      sha256 "{{SHA_DARWIN_ARM64}}"
    end
    on_intel do
      odie "ship is not available for Intel macOS. Apple Silicon (ARM64) is required."
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/kula-app/ship/releases/download/v{{VERSION}}/ship-linux-arm64"
      sha256 "{{SHA_LINUX_ARM64}}"
    end
    on_intel do
      url "https://github.com/kula-app/ship/releases/download/v{{VERSION}}/ship-linux-amd64"
      sha256 "{{SHA_LINUX_AMD64}}"
    end
  end

  def install
    binary = Dir["ship-*"].first
    bin.install binary => "ship"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/ship --version")
  end
end
