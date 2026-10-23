FROM python:3.13-alpine
WORKDIR /app
COPY server.py ./server.py
USER 65532:65532
EXPOSE 8090
CMD ["python", "server.py"]
