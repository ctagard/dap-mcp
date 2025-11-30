#
# spec file for package dap-mcp
#
# Copyright (c) 2024 Cole Agard
#
# All modifications and additions to the file contributed by third parties
# remain the property of their copyright owners, unless otherwise agreed
# upon. The license for this file, and modifications and additions to the
# file, is the same license as for the pristine package itself (MIT).
#

Name:           dap-mcp
Version:        0.1.0
Release:        1%{?dist}
Summary:        Debug Adapter Protocol server for AI agents via MCP

License:        MIT
URL:            https://github.com/ctagard/dap-mcp
Source0:        %{name}-%{version}.tar.xz

BuildRequires:  golang >= 1.22
BuildRequires:  git

%description
DAP-MCP is an MCP (Model Context Protocol) server that gives AI agents
the ability to debug code. Launch debug sessions, set breakpoints,
inspect variables, and step through code - all through natural language.

Supports debugging Go, Python, JavaScript, TypeScript, and browser-based
applications (React, Vue, Svelte).

%prep
%autosetup -n %{name}-%{version}

%build
export CGO_ENABLED=0
go build -mod=vendor -ldflags "-X main.version=%{version}" -o %{name} ./cmd/dap-mcp

%install
install -Dm755 %{name} %{buildroot}%{_bindir}/%{name}

%files
%license LICENSE
%doc README.md
%{_bindir}/%{name}

%changelog
* Sat Nov 30 2024 Cole Agard <cole.thomas.agard@gmail.com> - 0.1.0-1
- Initial package release
