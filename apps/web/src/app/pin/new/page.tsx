import NavBar from "@/components/nav/NavBar";
import PinCreateForm from "./PinCreateForm";
import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "작품 올리기 — Fugue",
};

export default function PinNewPage() {
  return (
    <>
      <NavBar />
      <main className="flex-1 max-w-2xl mx-auto w-full px-6 py-8">
        <PinCreateForm />
      </main>
    </>
  );
}
