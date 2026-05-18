export async function downloadPhoto(url: string, filename: string): Promise<void> {
  const blob = await fetchDownloadBlob(url, '下载失败');
  triggerDownload(blob, filename);
}

export async function downloadPhotosAsZip(
  items: { url: string; filename: string }[],
  zipName: string,
  onProgress: (current: number, total: number, currentName: string) => void,
  signal?: AbortSignal,
): Promise<void> {
  const { default: JSZip } = await import('jszip');
  const zip = new JSZip();

  for (let i = 0; i < items.length; i++) {
    if (signal?.aborted) {
      throw new DOMException('下载已取消', 'AbortError');
    }

    const item = items[i];
    onProgress(i + 1, items.length, item.filename);

    const blob = await fetchDownloadBlob(item.url, `下载 ${item.filename} 失败`, signal);
    zip.file(item.filename, blob);
  }

  const zipBlob = await zip.generateAsync({ type: 'blob' });
  triggerDownload(zipBlob, zipName);
}

async function fetchDownloadBlob(url: string, failurePrefix: string, signal?: AbortSignal) {
  let response: Response;
  try {
    response = await fetch(url, { signal });
  } catch (err) {
    if (err instanceof DOMException && err.name === 'AbortError') {
      throw err;
    }
    throw new Error(`${failurePrefix}，请检查网络或文件访问权限。`);
  }
  if (!response.ok) {
    throw new Error(`${failurePrefix} (${response.status})`);
  }
  return response.blob();
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
