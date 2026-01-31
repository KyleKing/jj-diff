# typed: false
# frozen_string_literal: true

# Homebrew formula for jj-diff
# Install: brew install --formula ./Formula/jj-diff.rb
# Update SHA256 values after each release using:
#   shasum -a 256 jj-diff-darwin-*

class JjDiff < Formula
  desc "Exploring better diff management for jj"
  homepage "https://github.com/kyleking/jj-diff"
  license "MIT"
  version "0.1.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/kyleking/jj-diff/releases/download/v#{version}/jj-diff-darwin-arm64"
      sha256 "REPLACE_WITH_SHA256_FOR_DARWIN_ARM64"
    else
      url "https://github.com/kyleking/jj-diff/releases/download/v#{version}/jj-diff-darwin-amd64"
      sha256 "REPLACE_WITH_SHA256_FOR_DARWIN_AMD64"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/kyleking/jj-diff/releases/download/v#{version}/jj-diff-linux-arm64"
      sha256 "REPLACE_WITH_SHA256_FOR_LINUX_ARM64"
    else
      url "https://github.com/kyleking/jj-diff/releases/download/v#{version}/jj-diff-linux-amd64"
      sha256 "REPLACE_WITH_SHA256_FOR_LINUX_AMD64"
    end
  end

  def install
    binary_name = "jj-diff-#{OS.kernel_name.downcase}-#{Hardware::CPU.arch}"
    bin.install binary_name => "jj-diff"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/jj-diff --version")
  end
end
