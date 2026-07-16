"use client";

export default function HideOnErrorImage({
  alt = "",
  ...props
}: React.ImgHTMLAttributes<HTMLImageElement>) {
  return (
    <img
      {...props}
      alt={alt}
      onError={(e) => {
        e.currentTarget.style.display = "none";
      }}
    />
  );
}
