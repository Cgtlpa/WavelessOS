# WavelessOS

WavelessOS is a minimal, source-based Linux distribution built for stability, daily driving, and gaming. It is a stripped-back system that stays out of your way and lets your hardware perform.

We use **OpenRC** for a fast init system, **XFCE** as the default desktop, and **LibreWolf** for a secure, lightweight browser out of the box.

The OS installs via a minimal ISO with a terminal installation script.

---

## The Wave Package Manager

WavelessOS features **Wave**, a custom package manager built for speed and stability. Packages are defined using simple, human-readable TOML files.

While WavelessOS is source-based, Wave supports **binary packages** for heavy software (like Firefox or Xorg) so you don't have to spend hours compiling massive codebases.

### Syntax

Instead of `sudo` or `doas`, we use `doit`—a lightweight alternative written in Rust.

* **Install a package:**
```bash
doit wave acquire <package>

```


* **Remove a package:**
```bash
doit wave annihilate <package>

```


* **Search repositories:**
```bash
doit wave find <package>

```



---

## WUR (Wave User Repository)

The **WUR** is our community-driven repository. It allows users to upload, maintain, and share their own TOML package recipes, keeping the core system microscopic while giving you access to all the software you need.

---

## Contributing

WavelessOS is under active development. Feel free to open a Pull Request or check out the issues if you want to help hack on the Wave package manager, optimize build scripts, or maintain packages.