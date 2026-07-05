#!/usr/bin/env bash
# genericize.sh — deterministic first-pass placeholder substitution over a
# staged blueprint template. Data-driven: rules live in placeholder-map.tsv
# beside this script (override: GENERICIZE_MAP env). The refresh pass runs
# this FIRST; the LLM hand-judges structure only. Rows apply longest-search-
# first so a longer, more specific search always wins over a shorter one it
# contains.
set -euo pipefail

usage() {
  echo "Usage: genericize.sh [-i] [file ...]" >&2
  echo "  no file args           -> filter stdin to stdout" >&2
  echo "  one or more files      -> transformed content to stdout" >&2
  echo "  -i file [file ...]     -> edit the named file(s) in place" >&2
  exit 2
}

in_place=0
args=()
for arg in "$@"; do
  case "$arg" in
    -i)
      in_place=1
      ;;
    -*)
      usage
      ;;
    *)
      args+=("$arg")
      ;;
  esac
done

if [[ $in_place -eq 1 && ${#args[@]} -eq 0 ]]; then
  usage
fi

script_dir="$(dirname "$(readlink -f "$0")")"
map_file="${GENERICIZE_MAP:-$script_dir/placeholder-map.tsv}"

if [[ ! -f "$map_file" ]]; then
  echo "genericize.sh: map file not found: $map_file" >&2
  exit 1
fi

run_perl() {
  local in_file="$1"
  local out_file="$2"
  perl -e '
    my ($map_file, $in_file, $out_file) = @ARGV;

    open(my $mfh, "<", $map_file) or die "cannot open map file: $map_file: $!";
    my @rules;
    while (my $line = <$mfh>) {
      chomp $line;
      next if $line =~ /^\s*#/;
      next if $line =~ /^\s*$/;
      my ($search, $replace, $mode) = split(/\t/, $line, 3);
      next unless defined $search && defined $replace && defined $mode;
      push @rules, { search => $search, replace => $replace, mode => $mode };
    }
    close $mfh;

    # Sort rules by search length DESCENDING (stable sort).
    @rules = sort { length($b->{search}) <=> length($a->{search}) } @rules;

    my $content;
    {
      local $/;
      if ($in_file eq "-") {
        $content = <STDIN>;
      } else {
        open(my $ifh, "<", $in_file) or die "cannot open input file: $in_file: $!";
        $content = <$ifh>;
        close $ifh;
      }
    }
    $content = "" unless defined $content;

    for my $rule (@rules) {
      my $s = $rule->{search};
      my $r = $rule->{replace};
      my $q = quotemeta($s);
      if ($rule->{mode} eq "word") {
        $content =~ s/\b$q\b/$r/g;
      } else {
        $content =~ s/$q/$r/g;
      }
    }

    if ($out_file eq "-") {
      print STDOUT $content;
    } else {
      open(my $ofh, ">", $out_file) or die "cannot open output file: $out_file: $!";
      print $ofh $content;
      close $ofh;
    }
  ' "$map_file" "$in_file" "$out_file"
}

if [[ ${#args[@]} -eq 0 ]]; then
  # No file args -> filter stdin to stdout.
  run_perl "-" "-"
  exit 0
fi

if [[ $in_place -eq 1 ]]; then
  for f in "${args[@]}"; do
    tmp_out="$(mktemp)"
    run_perl "$f" "$tmp_out"
    cat "$tmp_out" > "$f"
    rm -f "$tmp_out"
  done
else
  for f in "${args[@]}"; do
    run_perl "$f" "-"
  done
fi
