export type CardType = "image" | "audio" | "text" | "video";

const FIELD_TO_CARD: Record<string, CardType> = {
  "미술": "image",
  "음악": "audio",
  "시나리오 라이터": "text",
  "영상편집": "video",
  "프로그래밍": "image",
};

export function getCardType(field: string): CardType {
  return FIELD_TO_CARD[field] ?? "image";
}

const FIELD_LABELS: Record<string, string> = {
  "미술": "Illustration",
  "음악": "Music",
  "시나리오 라이터": "Writing",
  "영상편집": "Video",
  "프로그래밍": "Code",
};

export function getFieldLabel(field: string): string {
  return FIELD_LABELS[field] ?? field;
}
