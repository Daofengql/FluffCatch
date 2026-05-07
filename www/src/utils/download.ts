import JSZip from 'jszip';

export async function downloadPhoto(url: string, filename: string): Promise<void> {
  const response = await fetch(url);
  if (!response.ok) {
    throw new Error(`下载失败 (${response.status})`);
  }
  const blob = await response.blob();
  triggerDownload(blob, filename);
}

export async function downloadPhotosAsZip(
  items: { url: string; filename: string }[],
  zipName: string,
  onProgress: (current: number, total: number, currentName: string) => void,
  signal?: AbortSignal,
): Promise<void> {
  const zip = new JSZip();

  for (let i = 0; i < items.length; i++) {
    if (signal?.aborted) {
      throw new DOMException('下载已取消', 'AbortError');
    }

    const item = items[i];
    onProgress(i + 1, items.length, item.filename);

    const response = await fetch(item.url, { signal });
    if (!response.ok) {
      throw new Error(`下载 ${item.filename} 失败 (${response.status})`);
    }
    const blob = await response.blob();
    zip.file(item.filename, blob);
  }

  const zipBlob = await zip.generateAsync({ type: 'blob' });
  triggerDownload(zipBlob, zipName);
}

function triggerDownload(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = filename;
  document.body.appendChild(anchor);
  anchor.click();
  document.body.removeChild(anchor);
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
