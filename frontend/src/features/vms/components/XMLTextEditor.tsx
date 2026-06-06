import { type ReactNode, useEffect, useMemo, useRef, useState } from 'react';

import { ChevronDownIcon, ChevronLeftIcon, ChevronRightIcon, ChevronUpIcon, SearchIcon, XIcon } from 'lucide-react';

import { KvmTooltip } from '../../../components/kvm/StatusBadge';

export function XMLTextEditor({
  value,
  readOnly,
  disabled,
  className = '',
  heightClassName = 'h-[min(50vh,520px)]',
  onChange,
}: {
  value: string;
  readOnly?: boolean;
  disabled?: boolean;
  className?: string;
  heightClassName?: string;
  onChange: (value: string) => void;
}) {
  const [query, setQuery] = useState('');
  const [searchOpen, setSearchOpen] = useState(false);
  const [activeMatch, setActiveMatch] = useState(0);
  const [selectionMode, setSelectionMode] = useState(false);
  const [selectionTick, setSelectionTick] = useState(0);
  const [textareaScroll, setTextareaScroll] = useState({ left: 0, top: 0 });
  const searchInputRef = useRef<HTMLInputElement | null>(null);
  const textareaRef = useRef<HTMLTextAreaElement | null>(null);
  const matches = useMemo(() => findMatches(value, query), [query, value]);
  const matchCount = matches.length;
  const highlightedXML = useMemo(() => renderHighlightedXML(value, matches, activeMatch), [activeMatch, matches, value]);

  useEffect(() => {
    if (!searchOpen) return;
    searchInputRef.current?.focus({ preventScroll: true });
  }, [searchOpen]);

  useEffect(() => {
    if (!matchCount) {
      setActiveMatch(0);
      setSelectionMode(false);
      return;
    }
    const anchor = selectionMode ? textareaRef.current?.selectionStart || 0 : getTextareaSearchAnchor(textareaRef.current, value);
    setActiveMatch(findMatchIndexAtOrAfter(matches, anchor));
    setSelectionMode(true);
  }, [matches, matchCount, selectionMode, value]);

  useEffect(() => {
    if (!selectionTick) return;
    const match = matches[activeMatch];
    const textarea = textareaRef.current;
    if (!textarea || !match) return;
    textarea.setSelectionRange(match.start, match.end);
    scrollTextareaToMatch(textarea, value, match.start);
    setTextareaScroll({ left: textarea.scrollLeft, top: textarea.scrollTop });
    searchInputRef.current?.focus({ preventScroll: true });
  }, [activeMatch, matches, selectionTick, value]);

  function selectMatch(index: number) {
    if (!matchCount) return;
    setSelectionMode(true);
    setActiveMatch(((index % matchCount) + matchCount) % matchCount);
    setSelectionTick(tick => tick + 1);
  }

  function moveMatch(direction: 1 | -1) {
    if (!matchCount) return;
    selectMatch(activeMatch + direction);
  }

  return (
    <div className={`relative ${className}`}>
      <div className="absolute right-3 top-3 z-10 flex max-w-[calc(100%-1.5rem)] items-start gap-2">
        <KvmTooltip label={searchOpen ? '收起搜索' : '展开搜索'} placement="top">
          <button
            type="button"
            onClick={() => setSearchOpen(open => !open)}
            className="kvm-action-button flex h-10 w-9 items-center justify-center rounded-xl border shadow-lg backdrop-blur"
            style={{
              background: 'var(--kvm-popover-bg)',
              borderColor: searchOpen ? 'rgba(59,130,246,0.42)' : 'var(--kvm-popover-border)',
              color: searchOpen ? 'var(--kvm-accent-text)' : 'var(--kvm-text-muted)',
              boxShadow: 'var(--kvm-menu-shadow)',
            }}
            aria-label={searchOpen ? '收起搜索' : '展开搜索'}
            aria-expanded={searchOpen}
          >
            {searchOpen ? <ChevronRightIcon size={16} /> : <ChevronLeftIcon size={16} />}
          </button>
        </KvmTooltip>
        {searchOpen && (
          <div
            className="flex min-w-0 items-center gap-1.5 rounded-xl border px-2 py-1.5 shadow-lg backdrop-blur"
            style={{
              background: 'var(--kvm-popover-bg)',
              borderColor: 'var(--kvm-popover-border)',
              boxShadow: 'var(--kvm-menu-shadow)',
            }}
          >
            <SearchIcon size={14} className="shrink-0" style={{ color: 'var(--kvm-text-muted)' }} />
            <input
              ref={searchInputRef}
              value={query}
              onChange={event => setQuery(event.target.value)}
              onKeyDown={event => {
                if (event.key !== 'Enter') return;
                event.preventDefault();
                if (!matchCount) return;
                moveMatch(event.shiftKey ? -1 : 1);
              }}
              placeholder="搜索 XML"
              className="h-7 w-32 min-w-0 bg-transparent text-xs font-semibold outline-none sm:w-44"
              style={{ color: 'var(--kvm-text)' }}
              aria-label="搜索 XML"
            />
            <span
              className="w-14 shrink-0 text-center text-[11px] font-semibold"
              style={{ color: query ? 'var(--kvm-accent-text)' : 'var(--kvm-text-muted)' }}
            >
              {query ? (matchCount ? `${activeMatch + 1}/${matchCount}` : '0/0') : '-'}
            </span>
            <KvmTooltip label="上一个命中" placement="top">
              <button
                type="button"
                onClick={() => moveMatch(-1)}
                disabled={!matchCount}
                className="kvm-action-button flex h-7 w-7 items-center justify-center rounded-lg border disabled:opacity-45"
                style={{
                  background: 'var(--kvm-control-bg-soft)',
                  borderColor: 'var(--kvm-border)',
                  color: 'var(--kvm-text-muted)',
                }}
                aria-label="上一个命中"
              >
                <ChevronUpIcon size={14} />
              </button>
            </KvmTooltip>
            <KvmTooltip label="下一个命中" placement="top">
              <button
                type="button"
                onClick={() => moveMatch(1)}
                disabled={!matchCount}
                className="kvm-action-button flex h-7 w-7 items-center justify-center rounded-lg border disabled:opacity-45"
                style={{
                  background: 'var(--kvm-control-bg-soft)',
                  borderColor: 'var(--kvm-border)',
                  color: 'var(--kvm-text-muted)',
                }}
                aria-label="下一个命中"
              >
                <ChevronDownIcon size={14} />
              </button>
            </KvmTooltip>
            {query && (
              <KvmTooltip label="清空搜索" placement="top">
                <button
                  type="button"
                  onClick={() => setQuery('')}
                  className="kvm-action-button flex h-7 w-7 items-center justify-center rounded-lg border"
                  style={{
                    background: 'rgba(239,68,68,0.1)',
                    borderColor: 'rgba(248,113,113,0.28)',
                    color: '#fca5a5',
                  }}
                  aria-label="清空搜索"
                >
                  <XIcon size={14} />
                </button>
              </KvmTooltip>
            )}
          </div>
        )}
      </div>
      <div
        className={`relative overflow-hidden rounded-xl border ${heightClassName}`}
        style={{
          background: readOnly ? 'var(--kvm-control-bg-soft)' : 'var(--kvm-control-bg)',
          borderColor: readOnly ? 'var(--kvm-border)' : 'rgba(59,130,246,0.42)',
          boxShadow: readOnly
            ? 'inset 0 1px 0 rgba(255,255,255,0.05)'
            : '0 0 0 3px rgba(59,130,246,0.08), inset 0 1px 0 rgba(255,255,255,0.05)',
        }}
      >
        {query && matchCount > 0 && (
          <pre
            aria-hidden="true"
            className="kvm-xml-highlight-layer pointer-events-none absolute inset-0 overflow-hidden p-4 font-mono text-sm leading-6"
          >
            <code
              className="block min-h-full whitespace-pre-wrap break-words"
              style={{ transform: `translate(${-textareaScroll.left}px, ${-textareaScroll.top}px)` }}
            >
              {highlightedXML}
            </code>
          </pre>
        )}
        <textarea
          ref={textareaRef}
          readOnly={readOnly || disabled}
          value={value}
          onChange={event => onChange(event.target.value)}
          onScroll={event =>
            setTextareaScroll({
              left: event.currentTarget.scrollLeft,
              top: event.currentTarget.scrollTop,
            })
          }
          className="kvm-xml-textarea kvm-hidden-scrollbar relative z-[1] h-full w-full resize-none overflow-auto bg-transparent p-4 font-mono text-sm leading-6 outline-none disabled:opacity-75"
          style={{ color: 'var(--kvm-text)' }}
        />
      </div>
    </div>
  );
}

function scrollTextareaToMatch(textarea: HTMLTextAreaElement, text: string, index: number) {
  const style = window.getComputedStyle(textarea);
  const lineHeight = Number.parseFloat(style.lineHeight) || 24;
  const paddingTop = Number.parseFloat(style.paddingTop) || 0;
  const linesBefore = text.slice(0, index).split('\n').length - 1;
  const targetTop = Math.max(0, linesBefore * lineHeight + paddingTop - textarea.clientHeight / 3);
  textarea.scrollTop = targetTop;
}

function getTextareaSearchAnchor(textarea: HTMLTextAreaElement | null, text: string) {
  if (!textarea) return 0;
  if (textarea.selectionStart !== textarea.selectionEnd) return textarea.selectionEnd;
  if (textarea.selectionStart > 0) return textarea.selectionStart;

  const style = window.getComputedStyle(textarea);
  const lineHeight = Number.parseFloat(style.lineHeight) || 24;
  const visibleLine = Math.max(0, Math.floor(textarea.scrollTop / lineHeight));
  let line = 0;
  for (let index = 0; index < text.length; index += 1) {
    if (line >= visibleLine) return index;
    if (text[index] === '\n') line += 1;
  }
  return 0;
}

function findMatchIndexAtOrAfter(matches: Array<{ start: number; end: number }>, anchor: number) {
  const index = matches.findIndex(match => match.start >= anchor);
  return index === -1 ? 0 : index;
}

function findMatches(text: string, query: string) {
  const keyword = query.trim().toLowerCase();
  if (!keyword) return [];
  const source = text.toLowerCase();
  const matches: Array<{ start: number; end: number }> = [];
  let index = source.indexOf(keyword);
  while (index !== -1) {
    matches.push({ start: index, end: index + keyword.length });
    index = source.indexOf(keyword, index + keyword.length);
  }
  return matches;
}

function renderHighlightedXML(text: string, matches: Array<{ start: number; end: number }>, activeMatch: number) {
  if (!matches.length) return text;
  const nodes: ReactNode[] = [];
  let cursor = 0;
  matches.forEach((match, index) => {
    if (match.start > cursor) {
      nodes.push(text.slice(cursor, match.start));
    }
    nodes.push(
      <mark
        key={`${match.start}-${match.end}`}
        className={index === activeMatch ? 'kvm-xml-highlight-current' : 'kvm-xml-highlight-match'}
      >
        {text.slice(match.start, match.end)}
      </mark>
    );
    cursor = match.end;
  });
  if (cursor < text.length) {
    nodes.push(text.slice(cursor));
  }
  return nodes;
}
