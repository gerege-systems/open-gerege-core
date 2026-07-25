# Gerege Template Platform V3.0
# Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.
#
# Postgres + pgvector — AI-ийн мэдлэгийн сангийн семантик (вектор) хайлтад.
#
# Яагаад албан ёсны pgvector/pgvector:pg16 БИШ вэ: тэр image нь Debian (glibc)
# суурьтай. Одоо ажиллаж буй өгөгдөл alpine (musl) дээр үүссэн тул шилжихэд
# текстийн индексүүдийн collation хувилбар зөрж, Postgres анхааруулга өгөөд
# REINDEX шаардаж болзошгүй. Иймд суурь image-ээ хэвээр (postgres:16-alpine)
# үлдээж, зөвхөн extension-ыг эмхэтгэж нэмнэ — өгөгдөлд эрсдэлгүй.
FROM postgres:16-alpine

# pgvector-ийн хувилбарыг тогтмолжуулна (reproducible build). clang/llvm-ийн
# хувилбар нь суурь image-ийн Postgres-ийг эмхэтгэсэн LLVM-тэй таарах ёстой
# (PGXS нь JIT bitcode-д яг тэр clang-<хувилбар>-ыг дуудна).
ARG PGVECTOR_VERSION=0.8.0

RUN set -eux; \
    apk add --no-cache --virtual .build-deps build-base clang21 llvm21-dev llvm21 wget; \
    wget -q -O /tmp/pgvector.tar.gz \
        "https://github.com/pgvector/pgvector/archive/refs/tags/v${PGVECTOR_VERSION}.tar.gz"; \
    tar -xzf /tmp/pgvector.tar.gz -C /tmp; \
    cd "/tmp/pgvector-${PGVECTOR_VERSION}"; \
    make OPTFLAGS=""; \
    make install; \
    cd /; \
    rm -rf /tmp/pgvector*; \
    apk del .build-deps
