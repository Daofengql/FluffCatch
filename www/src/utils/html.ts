const allowedTags = new Set([
  'A',
  'B',
  'BR',
  'CODE',
  'EM',
  'H1',
  'H2',
  'H3',
  'H4',
  'H5',
  'H6',
  'I',
  'IMG',
  'LI',
  'OL',
  'P',
  'SMALL',
  'SPAN',
  'STRONG',
  'U',
  'UL'
]);

const blockedTags = new Set(['IFRAME', 'LINK', 'META', 'OBJECT', 'SCRIPT', 'STYLE']);

const allowedAttrs: Record<string, Set<string>> = {
  A: new Set(['href', 'title', 'target', 'rel']),
  IMG: new Set(['src', 'alt', 'title', 'width', 'height']),
  '*': new Set(['aria-label'])
};

export function sanitizeFooterHtml(value: string) {
  if (typeof document === 'undefined') {
    return value;
  }

  const template = document.createElement('template');
  template.innerHTML = value;
  sanitizeNode(template.content);
  return template.innerHTML;
}

function sanitizeNode(parent: ParentNode) {
  for (const node of Array.from(parent.childNodes)) {
    if (node.nodeType === Node.COMMENT_NODE) {
      node.remove();
      continue;
    }
    if (node.nodeType !== Node.ELEMENT_NODE) {
      continue;
    }

    const element = node as HTMLElement;
    if (blockedTags.has(element.tagName)) {
      element.remove();
      continue;
    }
    if (!allowedTags.has(element.tagName)) {
      element.replaceWith(...Array.from(element.childNodes));
      sanitizeNode(parent);
      continue;
    }

    sanitizeAttributes(element);
    sanitizeNode(element);
  }
}

function sanitizeAttributes(element: HTMLElement) {
  const allowed = allowedAttrs[element.tagName] ?? new Set<string>();
  const globalAllowed = allowedAttrs['*'];
  for (const attr of Array.from(element.attributes)) {
    const name = attr.name.toLowerCase();
    const isAllowed = allowed.has(attr.name) || globalAllowed.has(attr.name);
    if (!isAllowed || name.startsWith('on') || name === 'style') {
      element.removeAttribute(attr.name);
    }
  }

  if (element instanceof HTMLAnchorElement) {
    sanitizeAnchor(element);
  }
  if (element instanceof HTMLImageElement) {
    sanitizeImage(element);
  }
}

function sanitizeAnchor(anchor: HTMLAnchorElement) {
  const href = anchor.getAttribute('href') || '';
  if (!isAllowedUrl(href)) {
    anchor.removeAttribute('href');
  }
  const target = anchor.getAttribute('target');
  if (target && target !== '_blank') {
    anchor.removeAttribute('target');
  }
  if (anchor.getAttribute('target') === '_blank') {
    anchor.setAttribute('rel', 'noopener noreferrer');
  }
}

function sanitizeImage(image: HTMLImageElement) {
  const src = image.getAttribute('src') || '';
  if (!isAllowedUrl(src)) {
    image.remove();
  }
}

function isAllowedUrl(value: string) {
  const trimmed = value.trim();
  if (!trimmed) return false;
  if (trimmed.startsWith('/') && !trimmed.startsWith('//')) return true;
  if (trimmed.startsWith('#')) return true;
  try {
    const parsed = new URL(trimmed);
    return ['http:', 'https:', 'mailto:', 'tel:'].includes(parsed.protocol);
  } catch {
    return false;
  }
}
