import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { CameraIcon, Loader2Icon } from "lucide-react";
import { useRef } from "react";
import { toast } from "sonner";
import { uploadReceipt } from "@/api/gen/sdk.gen";
import { Button } from "@/components/ui/button";
import { errorText } from "@/lib/api";
import { cn } from "@/lib/utils";
import { m } from "@/paraglide/messages";

/** Uploads files into the receipt inbox; returns the created receipts. */
export function useUploadReceipts() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (files: File[]) => {
      const results = [];
      for (const file of files) {
        const { data } = await uploadReceipt({
          query: { filename: file.name },
          body: file,
          throwOnError: true,
        });
        results.push(data);
      }
      return results;
    },
    onSuccess: (r) => {
      qc.invalidateQueries({ queryKey: [{ _id: "listReceipts" }] });
      qc.invalidateQueries({ queryKey: [{ _id: "getDashboard" }] });
      toast.success(m.receipt_uploaded({ count: String(r.length) }));
    },
    onError: (err) => toast.error(errorText(err)),
  });
}

/** Camera/file capture button. On phones `capture` opens the camera. */
export function ReceiptCapture({
  iconOnly,
  variant = "default",
  className,
  onUploaded,
}: {
  iconOnly?: boolean;
  variant?: "default" | "outline";
  className?: string;
  onUploaded?: (ids: number[]) => void;
}) {
  const input = useRef<HTMLInputElement>(null);
  const upload = useUploadReceipts();
  const navigate = useNavigate();
  const onFiles = (files: FileList | null) => {
    if (!files?.length) return;
    upload.mutate(Array.from(files), {
      onSuccess: (r) => {
        const ids = r.map((x) => x.id);
        if (onUploaded) onUploaded(ids);
        else navigate({ to: "/receipts", search: { focus: ids[0] } });
      },
    });
    if (input.current) input.current.value = "";
  };
  const icon = upload.isPending ? <Loader2Icon className="animate-spin" /> : <CameraIcon />;
  return (
    <>
      <input
        ref={input}
        type="file"
        accept="image/jpeg,image/png,application/pdf"
        capture="environment"
        multiple
        className="sr-only"
        tabIndex={-1}
        aria-hidden
        onChange={(e) => onFiles(e.target.files)}
      />
      {iconOnly ? (
        <button
          type="button"
          onClick={() => input.current?.click()}
          disabled={upload.isPending}
          aria-label={m.capture_receipt()}
          className="-mt-5 grid size-14 place-items-center rounded-full bg-primary text-primary-foreground shadow-lg ring-4 ring-background [&_svg]:size-6"
        >
          {icon}
        </button>
      ) : (
        <Button
          variant={variant}
          className={cn(className)}
          onClick={() => input.current?.click()}
          disabled={upload.isPending}
        >
          {icon}
          {m.capture_receipt()}
        </Button>
      )}
    </>
  );
}

/** Desktop file picker without camera hint. */
export function ReceiptFilePicker({ onUploaded }: { onUploaded?: (ids: number[]) => void }) {
  const input = useRef<HTMLInputElement>(null);
  const upload = useUploadReceipts();
  return (
    <>
      <input
        ref={input}
        type="file"
        accept="image/jpeg,image/png,application/pdf"
        multiple
        className="sr-only"
        tabIndex={-1}
        aria-hidden
        onChange={(e) => {
          if (e.target.files?.length)
            upload.mutate(Array.from(e.target.files), { onSuccess: (r) => onUploaded?.(r.map((x) => x.id)) });
          e.target.value = "";
        }}
      />
      <Button variant="outline" onClick={() => input.current?.click()} disabled={upload.isPending}>
        {upload.isPending ? <Loader2Icon className="animate-spin" /> : null}
        {m.upload_files()}
      </Button>
    </>
  );
}
