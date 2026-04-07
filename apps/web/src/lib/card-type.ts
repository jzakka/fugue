export type CardType = "image" | "audio" | "text" | "video";

export function getCardType(mediaType: string): CardType {
  switch (mediaType) {
    case "audio":
      return "audio";
    case "video":
      return "video";
    case "image":
    default:
      return "image";
  }
}

const MEDIA_TYPE_LABELS: Record<string, string> = {
  image: "Image",
  audio: "Music",
  video: "Video",
};

export function getMediaTypeLabel(mediaType: string): string {
  return MEDIA_TYPE_LABELS[mediaType] ?? mediaType;
}
