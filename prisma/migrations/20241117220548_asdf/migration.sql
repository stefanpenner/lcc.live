-- CreateTable
CREATE TABLE "Cameras" (
    "id" TEXT NOT NULL,
    "kind" TEXT NOT NULL,
    "alt" TEXT NOT NULL,
    "src" TEXT NOT NULL,
    "canyon" TEXT NOT NULL,
    "host" TEXT NOT NULL
);

-- CreateIndex
CREATE UNIQUE INDEX "Cameras_id_key" ON "Cameras"("id");
