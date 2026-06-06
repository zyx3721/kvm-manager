export function domainNameFromXML(xml: string) {
  const text = xml.trim();
  if (!text) return '';
  try {
    const doc = new DOMParser().parseFromString(text, 'application/xml');
    if (doc.querySelector('parsererror')) return '';
    return doc.querySelector('domain > name')?.textContent?.trim() || '';
  } catch {
    return '';
  }
}
