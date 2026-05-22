import { redirect } from "next/navigation";
import NavBar from "@/components/nav/NavBar";
import { getAuthUser } from "@/lib/auth";
import PinCreateForm from "./PinCreateForm";
import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "핀 생성 — Fugue",
};

export default async function PinNewPage() {
  const user = await getAuthUser();
  if (!user) {
    redirect("/login");
  }

  return (
    <>
      <NavBar />
      <main id="main" className="flex-1 max-w-2xl mx-auto w-full px-6 py-8">
        <PinCreateForm />
      </main>
    </>
  );
}
